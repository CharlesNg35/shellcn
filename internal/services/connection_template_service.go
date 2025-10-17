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
	protocolID = strings.TrimSpace(protocolID)
	if protocolID == "" {
		return nil, nil
	}

	var record models.ConnectionTemplate
	err := s.db.WithContext(ctx).
		Where("driver_id = ?", protocolID).
		Order("created_at DESC").
		First(&record).Error
	if err == nil {
		return modelToDriverTemplate(record)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("connection template service: load template: %w", err)
	}

	if s.registry == nil {
		return nil, nil
	}

	driver, ok := s.registry.Get(protocolID)
	if !ok {
		return nil, nil
	}
	templater, ok := driver.(drivers.ConnectionTemplater)
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
	if err := validateConnectionTemplate(template); err != nil {
		return nil, fmt.Errorf("connection template service: driver template invalid: %w", err)
	}
	if err := s.persist(ctx, driver.ID(), template); err != nil {
		return nil, err
	}
	return template, nil
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

func (s *ConnectionTemplateService) persist(ctx context.Context, fallbackDriverID string, template *drivers.ConnectionTemplate) error {
	driverID := strings.TrimSpace(template.DriverID)
	if driverID == "" {
		driverID = strings.TrimSpace(fallbackDriverID)
	}
	version := strings.TrimSpace(template.Version)
	if version == "" {
		return fmt.Errorf("connection template service: template for %s missing version", driverID)
	}

	sectionsJSON, err := json.Marshal(template.Sections)
	if err != nil {
		return fmt.Errorf("connection template service: marshal sections: %w", err)
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
		Metadata:    metadataJSON,
		Hash:        hash,
	}

	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "driver_id"}, {Name: "version"}},
		DoUpdates: clause.AssignmentColumns([]string{"display_name", "description", "sections", "metadata", "hash"}),
	}).Create(&record).Error; err != nil {
		return fmt.Errorf("connection template service: upsert template: %w", err)
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

	return &drivers.ConnectionTemplate{
		DriverID:    record.DriverID,
		Version:     record.Version,
		DisplayName: record.DisplayName,
		Description: record.Description,
		Sections:    sections,
		Metadata:    metadata,
	}, nil
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
