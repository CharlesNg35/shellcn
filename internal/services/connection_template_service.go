package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/models"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

// ConnectionTemplateService resolves driver templates and materialises field payloads into persisted models.
type ConnectionTemplateService struct {
	db       *gorm.DB
	registry *drivers.Registry
}

// MaterialisedConnection represents the result of applying a template to user-provided fields.
type MaterialisedConnection struct {
	Settings        map[string]any
	Metadata        map[string]any
	Targets         []models.ConnectionTarget
	Fields          map[string]any
	TemplateVersion string
}

// ConnectionConfig describes the normalised configuration used by session launch handlers.
type ConnectionConfig struct {
	Settings map[string]any
	Metadata map[string]any
	Targets  []models.ConnectionTarget
}

// NewConnectionTemplateService constructs a ConnectionTemplateService instance.
func NewConnectionTemplateService(db *gorm.DB, registry *drivers.Registry) (*ConnectionTemplateService, error) {
	if db == nil {
		return nil, errors.New("connection template service: db is required")
	}
	return &ConnectionTemplateService{db: db, registry: registry}, nil
}

// Resolve retrieves the latest template for the supplied protocol, falling back to the driver registry when needed.
func (s *ConnectionTemplateService) Resolve(ctx context.Context, protocolID string) (*drivers.ConnectionTemplate, error) {
	if s == nil {
		return nil, nil
	}

	ctx = ensureContext(ctx)
	protocolID = strings.TrimSpace(strings.ToLower(protocolID))
	if protocolID == "" {
		return nil, nil
	}

	if template, err := s.resolveFromBinding(ctx, protocolID); err != nil {
		return nil, err
	} else if template != nil {
		return template, nil
	}

	if template, err := s.resolveByDriver(ctx, protocolID); err != nil {
		return nil, err
	} else if template != nil {
		return template, nil
	}

	if s.registry == nil {
		return nil, nil
	}

	return s.resolveFromRegistry(ctx, protocolID)
}

func (s *ConnectionTemplateService) resolveFromBinding(ctx context.Context, protocolID string) (*drivers.ConnectionTemplate, error) {
	if s.db == nil {
		return nil, nil
	}

	var binding models.ConnectionTemplateProtocol
	err := s.db.WithContext(ctx).
		Where("protocol_id = ?", protocolID).
		First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("connection template service: load binding: %w", err)
	}

	template, err := s.loadTemplateRecord(ctx, binding.DriverID, binding.Version)
	if err != nil {
		return nil, err
	}
	if template != nil {
		return template, nil
	}

	if binding.DriverID != "" {
		return s.loadTemplateRecord(ctx, binding.DriverID, "")
	}

	return nil, nil
}

func (s *ConnectionTemplateService) resolveByDriver(ctx context.Context, driverID string) (*drivers.ConnectionTemplate, error) {
	template, err := s.loadTemplateRecord(ctx, driverID, "")
	if err != nil || template == nil {
		return template, err
	}
	if len(template.Protocols) == 0 {
		template.Protocols = models.NormalizeConnectionTemplateProtocols(nil, driverID)
		if err := s.persist(ctx, driverID, template); err != nil {
			return nil, err
		}
	}
	return template, nil
}

func (s *ConnectionTemplateService) resolveFromRegistry(ctx context.Context, protocolID string) (*drivers.ConnectionTemplate, error) {
	if s.registry == nil {
		return nil, nil
	}

	if driver, ok := s.registry.Get(protocolID); ok {
		template, err := s.fetchTemplateFromDriver(ctx, driver, protocolID)
		if err != nil {
			return nil, err
		}
		if template != nil {
			return template, nil
		}
	}

	for _, drv := range s.registry.All() {
		if strings.EqualFold(drv.ID(), protocolID) {
			continue
		}
		template, err := s.fetchTemplateFromDriver(ctx, drv, protocolID)
		if err != nil {
			return nil, err
		}
		if template != nil {
			return template, nil
		}
	}

	return nil, nil
}

func (s *ConnectionTemplateService) fetchTemplateFromDriver(ctx context.Context, drv drivers.Driver, protocolID string) (*drivers.ConnectionTemplate, error) {
	templater, ok := drv.(drivers.ConnectionTemplater)
	if !ok {
		return nil, nil
	}

	template, err := templater.ConnectionTemplate()
	if err != nil {
		return nil, fmt.Errorf("connection template service: driver template: %w", err)
	}
	if template == nil {
		return nil, nil
	}

	protocols := models.NormalizeConnectionTemplateProtocols(template.Protocols, drv.ID())
	template.Protocols = protocols

	if !containsProtocol(protocols, protocolID) {
		return nil, nil
	}

	if err := validateConnectionTemplate(template); err != nil {
		return nil, fmt.Errorf("connection template service: driver template invalid: %w", err)
	}

	if err := s.persist(ctx, drv.ID(), template); err != nil {
		return nil, err
	}

	return template, nil
}

func (s *ConnectionTemplateService) loadTemplateRecord(ctx context.Context, driverID, version string) (*drivers.ConnectionTemplate, error) {
	if s.db == nil {
		return nil, nil
	}

	driverID = strings.TrimSpace(strings.ToLower(driverID))
	if driverID == "" {
		return nil, nil
	}

	query := s.db.WithContext(ctx).Where("driver_id = ?", driverID)
	if strings.TrimSpace(version) != "" {
		query = query.Where("version = ?", strings.TrimSpace(version))
	}
	query = query.Order("created_at DESC")

	var record models.ConnectionTemplate
	if err := query.First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("connection template service: load template: %w", err)
	}

	return modelToDriverTemplate(record)
}

// Materialise transforms template fields into settings, metadata, and connection targets.
func (s *ConnectionTemplateService) Materialise(template *drivers.ConnectionTemplate, fields map[string]any) (*MaterialisedConnection, error) {
	if template == nil {
		return nil, nil
	}

	result := &MaterialisedConnection{
		Settings:        map[string]any{},
		Metadata:        map[string]any{},
		Fields:          map[string]any{},
		TemplateVersion: strings.TrimSpace(template.Version),
	}

	valueStore := make(map[string]any)
	if fields == nil {
		fields = map[string]any{}
	}

	targetBuilders := map[int]*targetAccumulator{}

	for _, section := range template.Sections {
		for _, field := range section.Fields {
			key := strings.TrimSpace(field.Key)
			if key == "" {
				continue
			}

			rawValue, provided := fields[key]
			if !provided || rawValue == nil {
				if field.Default != nil {
					rawValue = field.Default
					provided = true
				}
			}

			resolvedValue, hasValue, err := coerceFieldValue(field, rawValue, provided)
			if err != nil {
				return nil, err
			}

			if hasValue {
				valueStore[key] = resolvedValue
				result.Fields[key] = resolvedValue
			} else {
				result.Fields[key] = nil
			}

			if !dependenciesSatisfied(field.Dependencies, valueStore) {
				continue
			}

			if !hasValue {
				if field.Required {
					return nil, fieldError(key, "required", fmt.Sprintf("%s is required", field.Label))
				}
				continue
			}

			if err := applyBinding(result, targetBuilders, field, resolvedValue); err != nil {
				return nil, err
			}
		}
	}

	targets, err := collapseTargets(targetBuilders)
	if err != nil {
		return nil, err
	}
	result.Targets = targets

	return result, nil
}

// MaterialiseConfig reconstructs a connection's runtime configuration using stored template metadata and persisted fields.
func (s *ConnectionTemplateService) MaterialiseConfig(ctx context.Context, conn ConnectionDTO) (*ConnectionConfig, error) {
	settings := cloneAnyMap(conn.Settings)
	if settings == nil {
		settings = map[string]any{}
	}

	metadata := cloneAnyMap(conn.Metadata)
	targets := dtoTargetsToModels(conn.Targets)

	fields := map[string]any{}
	if metadata != nil {
		if raw, ok := metadata["connection_template"]; ok {
			if meta, ok := raw.(map[string]any); ok {
				if f, ok := meta["fields"].(map[string]any); ok {
					fields = cloneAnyMap(f)
				}
			}
		}
	}

	var template *drivers.ConnectionTemplate
	var err error
	if s != nil {
		template, err = s.Resolve(ctx, conn.ProtocolID)
		if err != nil {
			return nil, err
		}
	}

	if template != nil && len(fields) > 0 {
		materialised, err := s.Materialise(template, fields)
		if err != nil {
			return nil, err
		}
		if materialised != nil {
			settings = mergeAnyMaps(settings, materialised.Settings)
			metadata = mergeAnyMaps(materialised.Metadata, metadata)
			if len(materialised.Targets) > 0 {
				targets = cloneTargets(materialised.Targets)
			}
		}
	}

	host := ""
	port := 0
	if len(targets) > 0 {
		host = strings.TrimSpace(targets[0].Host)
		port = targets[0].Port
	}

	if host == "" {
		host = strings.TrimSpace(fmt.Sprint(settings["host"]))
	}
	if host == "" && len(fields) > 0 {
		if value, ok := fields["host"]; ok {
			host = strings.TrimSpace(fmt.Sprint(value))
		}
	}
	if host == "" {
		return nil, fieldError("host", "required", "Host is required")
	}

	if port <= 0 {
		if value, ok := settings["port"]; ok {
			if parsed, err := toInt(value); err == nil {
				port = parsed
			}
		}
	}
	if port <= 0 && len(fields) > 0 {
		if value, ok := fields["port"]; ok {
			if parsed, err := toInt(value); err == nil {
				port = parsed
			}
		}
	}
	if port <= 0 {
		return nil, fieldError("port", "invalid", "Port is required")
	}

	if len(targets) == 0 {
		targets = []models.ConnectionTarget{{Ordering: 0}}
	}
	targets[0].Host = host
	targets[0].Port = port

	settings["host"] = host
	settings["port"] = port

	return &ConnectionConfig{
		Settings: settings,
		Metadata: metadata,
		Targets:  targets,
	}, nil
}

func (s *ConnectionTemplateService) persist(ctx context.Context, fallbackDriverID string, template *drivers.ConnectionTemplate) error {
	driverID := strings.TrimSpace(template.DriverID)
	if driverID == "" {
		driverID = strings.TrimSpace(fallbackDriverID)
	}
	template.DriverID = driverID
	version := strings.TrimSpace(template.Version)
	if version == "" {
		return fmt.Errorf("connection template service: template for %s missing version", driverID)
	}

	sectionsJSON, err := json.Marshal(template.Sections)
	if err != nil {
		return fmt.Errorf("connection template service: marshal sections: %w", err)
	}

	protocols := models.NormalizeConnectionTemplateProtocols(template.Protocols, driverID)
	template.Protocols = protocols

	protocolsJSON, err := json.Marshal(protocols)
	if err != nil {
		return fmt.Errorf("connection template service: marshal protocols: %w", err)
	}

	var metadataJSON datatypes.JSON
	if template.Metadata != nil {
		data, err := json.Marshal(template.Metadata)
		if err != nil {
			return fmt.Errorf("connection template service: marshal metadata: %w", err)
		}
		metadataJSON = data
	}

	payload := map[string]any{
		"driver_id":    driverID,
		"version":      version,
		"display_name": strings.TrimSpace(template.DisplayName),
		"description":  strings.TrimSpace(template.Description),
		"sections":     template.Sections,
		"protocols":    protocols,
		"metadata":     template.Metadata,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("connection template service: encode payload: %w", err)
	}
	sum := sha256.Sum256(encoded)
	hash := hex.EncodeToString(sum[:])

	record := models.ConnectionTemplate{
		DriverID:    driverID,
		Version:     version,
		DisplayName: strings.TrimSpace(template.DisplayName),
		Description: strings.TrimSpace(template.Description),
		Sections:    sectionsJSON,
		Protocols:   datatypes.JSON(protocolsJSON),
		Metadata:    metadataJSON,
		Hash:        hash,
	}

	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "driver_id"}, {Name: "version"}},
		DoUpdates: clause.AssignmentColumns([]string{"display_name", "description", "sections", "protocols", "metadata", "hash"}),
	}).Create(&record).Error; err != nil {
		return fmt.Errorf("connection template service: upsert template: %w", err)
	}

	if err := s.syncTemplateProtocols(ctx, driverID, version, protocols); err != nil {
		return err
	}
	return nil
}

func (s *ConnectionTemplateService) syncTemplateProtocols(ctx context.Context, driverID, version string, protocols []string) error {
	if s == nil || s.db == nil {
		return nil
	}

	normalized := models.NormalizeConnectionTemplateProtocols(protocols, driverID)
	if len(normalized) == 0 {
		return nil
	}

	tx := s.db.WithContext(ctx)

	if err := tx.Where("driver_id = ? AND protocol_id NOT IN ?", driverID, normalized).
		Delete(&models.ConnectionTemplateProtocol{}).Error; err != nil {
		return fmt.Errorf("connection template service: prune template protocols: %w", err)
	}

	for _, protocolID := range normalized {
		record := models.ConnectionTemplateProtocol{
			ProtocolID: protocolID,
			DriverID:   driverID,
			Version:    version,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "protocol_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"driver_id", "version", "updated_at"}),
		}).Create(&record).Error; err != nil {
			return fmt.Errorf("connection template service: upsert template protocol %s: %w", protocolID, err)
		}
	}

	return nil
}

func modelToDriverTemplate(record models.ConnectionTemplate) (*drivers.ConnectionTemplate, error) {
	var sections []drivers.ConnectionSection
	if err := json.Unmarshal(record.Sections, &sections); err != nil {
		return nil, fmt.Errorf("connection template service: decode sections: %w", err)
	}

	var metadata map[string]any
	if len(record.Metadata) > 0 {
		if err := json.Unmarshal(record.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("connection template service: decode metadata: %w", err)
		}
	}

	protocols := []string{}
	if len(record.Protocols) > 0 {
		if err := json.Unmarshal(record.Protocols, &protocols); err != nil {
			return nil, fmt.Errorf("connection template service: decode protocols: %w", err)
		}
	}
	if len(protocols) == 0 && strings.TrimSpace(record.DriverID) != "" {
		protocols = []string{strings.TrimSpace(record.DriverID)}
	}

	return &drivers.ConnectionTemplate{
		DriverID:    record.DriverID,
		Version:     record.Version,
		DisplayName: record.DisplayName,
		Description: record.Description,
		Protocols:   protocols,
		Sections:    sections,
		Metadata:    metadata,
	}, nil
}

func containsProtocol(protocols []string, target string) bool {
	target = strings.TrimSpace(strings.ToLower(target))
	if target == "" {
		return false
	}
	for _, id := range protocols {
		if strings.EqualFold(strings.TrimSpace(id), target) {
			return true
		}
	}
	return false
}

func dtoTargetsToModels(dtos []ConnectionTargetDTO) []models.ConnectionTarget {
	if len(dtos) == 0 {
		return nil
	}
	targets := make([]models.ConnectionTarget, 0, len(dtos))
	for _, dto := range dtos {
		target := models.ConnectionTarget{
			Host:     strings.TrimSpace(dto.Host),
			Port:     dto.Port,
			Ordering: dto.Order,
		}
		if len(dto.Labels) > 0 {
			if data, err := json.Marshal(dto.Labels); err == nil {
				target.Labels = datatypes.JSON(data)
			}
		}
		targets = append(targets, target)
	}
	return targets
}

func coerceFieldValue(field drivers.ConnectionField, raw any, provided bool) (any, bool, error) {
	if !provided {
		return nil, false, nil
	}

	switch field.Type {
	case drivers.ConnectionFieldTypeString, drivers.ConnectionFieldTypeMultiline, drivers.ConnectionFieldTypeTargetHost:
		value := strings.TrimSpace(fmt.Sprint(raw))
		if err := validateString(field, value); err != nil {
			return nil, false, err
		}
		return value, true, nil
	case drivers.ConnectionFieldTypeBoolean:
		value, err := toBool(raw)
		if err != nil {
			return nil, false, fieldError(field.Key, "invalid", err.Error())
		}
		return value, true, nil
	case drivers.ConnectionFieldTypeNumber, drivers.ConnectionFieldTypeTargetPort:
		val, err := toInt(raw)
		if err != nil {
			return nil, false, fieldError(field.Key, "invalid", err.Error())
		}
		if err := validateNumber(field, val); err != nil {
			return nil, false, err
		}
		return val, true, nil
	case drivers.ConnectionFieldTypeSelect:
		value := strings.TrimSpace(fmt.Sprint(raw))
		if err := validateSelect(field, value); err != nil {
			return nil, false, err
		}
		return value, true, nil
	case drivers.ConnectionFieldTypeJSON:
		value, err := toJSONMap(raw)
		if err != nil {
			return nil, false, fieldError(field.Key, "invalid", err.Error())
		}
		return value, true, nil
	default:
		return raw, true, nil
	}
}

func validateString(field drivers.ConnectionField, value string) error {
	if pattern, ok := field.Validation["pattern"].(string); ok && pattern != "" {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fieldError(field.Key, "invalid", "validation pattern is invalid")
		}
		if !re.MatchString(value) {
			return fieldError(field.Key, "invalid", fmt.Sprintf("%s is not formatted correctly", field.Label))
		}
	}
	if max, ok := numericFromValidation(field.Validation["max_length"]); ok && max >= 0 {
		if len(value) > int(max) {
			return fieldError(field.Key, "invalid", fmt.Sprintf("%s must be at most %d characters", field.Label, int(max)))
		}
	}
	if min, ok := numericFromValidation(field.Validation["min_length"]); ok && min >= 0 {
		if len(value) < int(min) {
			return fieldError(field.Key, "invalid", fmt.Sprintf("%s must be at least %d characters", field.Label, int(min)))
		}
	}
	return nil
}

func validateNumber(field drivers.ConnectionField, value int) error {
	if max, ok := numericFromValidation(field.Validation["max"]); ok {
		if float64(value) > max {
			return fieldError(field.Key, "invalid", fmt.Sprintf("%s must be less than or equal to %d", field.Label, int(max)))
		}
	}
	if min, ok := numericFromValidation(field.Validation["min"]); ok {
		if float64(value) < min {
			return fieldError(field.Key, "invalid", fmt.Sprintf("%s must be greater than or equal to %d", field.Label, int(min)))
		}
	}
	return nil
}

func validateSelect(field drivers.ConnectionField, value string) error {
	if len(field.Options) == 0 {
		return fieldError(field.Key, "invalid", fmt.Sprintf("%s has no selectable options", field.Label))
	}
	for _, option := range field.Options {
		if option.Value == value {
			return nil
		}
	}
	return fieldError(field.Key, "invalid", fmt.Sprintf("%s must be one of the allowed options", field.Label))
}

func toBool(raw any) (bool, error) {
	switch v := raw.(type) {
	case bool:
		return v, nil
	case string:
		switch strings.TrimSpace(strings.ToLower(v)) {
		case "true", "1", "yes", "y":
			return true, nil
		case "false", "0", "no", "n", "":
			return false, nil
		default:
			return false, fmt.Errorf("value %q must be true or false", v)
		}
	case float64:
		return v != 0, nil
	case int:
		return v != 0, nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return false, err
		}
		return i != 0, nil
	default:
		return false, fmt.Errorf("value %v must be boolean", raw)
	}
}

func toInt(raw any) (int, error) {
	switch v := raw.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, err
		}
		return int(i), nil
	case string:
		number := strings.TrimSpace(v)
		if number == "" {
			return 0, nil
		}
		i, err := strconv.Atoi(number)
		if err != nil {
			return 0, err
		}
		return i, nil
	default:
		return 0, fmt.Errorf("value %v must be numeric", raw)
	}
}

func toJSONMap(raw any) (map[string]any, error) {
	switch v := raw.(type) {
	case map[string]any:
		return v, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return map[string]any{}, nil
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, err
		}
		return out, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
}

func fieldError(key, reason, message string) *apperrors.AppError {
	code := fmt.Sprintf("field.%s.%s", key, reason)
	return apperrors.New(code, message, http.StatusBadRequest)
}

func dependenciesSatisfied(deps []drivers.FieldDependency, values map[string]any) bool {
	if len(deps) == 0 {
		return true
	}
	for _, dep := range deps {
		key := strings.TrimSpace(dep.Field)
		if key == "" {
			continue
		}
		val, ok := values[key]
		if !ok {
			return false
		}
		if dep.Equals != nil {
			if !compareValues(val, dep.Equals) {
				return false
			}
		}
	}
	return true
}

func compareValues(actual, expected any) bool {
	normalizedActual := normalizeComparable(actual)
	normalizedExpected := normalizeComparable(expected)
	return reflect.DeepEqual(normalizedActual, normalizedExpected)
}

func normalizeComparable(value any) any {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.TrimSpace(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
		return strings.TrimSpace(v.String())
	default:
		return value
	}
}

type targetAccumulator struct {
	target models.ConnectionTarget
	extras map[string]string
}

func applyBinding(result *MaterialisedConnection, targets map[int]*targetAccumulator, field drivers.ConnectionField, value any) error {
	if field.Binding == nil {
		return nil
	}
	switch field.Binding.Target {
	case drivers.BindingTargetSettings:
		if result.Settings == nil {
			result.Settings = map[string]any{}
		}
		setNestedValue(result.Settings, field.Binding.Path, value)
	case drivers.BindingTargetMetadata:
		if result.Metadata == nil {
			result.Metadata = map[string]any{}
		}
		setNestedValue(result.Metadata, field.Binding.Path, value)
	case drivers.BindingTargetConnectionTarget:
		acc := targets[field.Binding.Index]
		if acc == nil {
			acc = &targetAccumulator{
				target: models.ConnectionTarget{
					Ordering: field.Binding.Index,
				},
				extras: map[string]string{},
			}
			targets[field.Binding.Index] = acc
		}
		switch strings.ToLower(field.Binding.Property) {
		case "host":
			acc.target.Host = strings.TrimSpace(fmt.Sprint(value))
		case "port":
			port, err := toInt(value)
			if err != nil {
				return fieldError(field.Key, "invalid", "port must be numeric")
			}
			acc.target.Port = port
		default:
			acc.extras[field.Binding.Property] = fmt.Sprint(value)
		}
	default:
		// Unsupported binding target: ignore silently for forward compatibility.
	}
	return nil
}

func setNestedValue(root map[string]any, path string, value any) {
	if root == nil || strings.TrimSpace(path) == "" {
		return
	}
	parts := strings.Split(path, ".")
	last := len(parts) - 1
	cursor := root
	for i, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			return
		}
		if i == last {
			cursor[key] = value
			return
		}
		next, ok := cursor[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			cursor[key] = next
		}
		cursor = next
	}
}

func collapseTargets(builders map[int]*targetAccumulator) ([]models.ConnectionTarget, error) {
	if len(builders) == 0 {
		return nil, nil
	}

	indices := make([]int, 0, len(builders))
	for idx := range builders {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	targets := make([]models.ConnectionTarget, 0, len(indices))
	for _, idx := range indices {
		builder := builders[idx]
		target := builder.target
		if len(builder.extras) > 0 {
			data, err := json.Marshal(builder.extras)
			if err != nil {
				return nil, err
			}
			target.Labels = datatypes.JSON(data)
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func numericFromValidation(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
