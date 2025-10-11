package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/auth/providers"
	"github.com/charlesng35/shellcn/internal/models"
)

// LDAPUserSyncResult captures membership changes performed for a single user.
type LDAPUserSyncResult struct {
	UserID             string `json:"user_id"`
	TeamsCreated       int    `json:"teams_created"`
	MembershipsAdded   int    `json:"memberships_added"`
	MembershipsRemoved int    `json:"memberships_removed"`
}

// LDAPSyncSummary aggregates the outcome of a bulk LDAP synchronisation.
type LDAPSyncSummary struct {
	UsersCreated       int `json:"users_created"`
	UsersUpdated       int `json:"users_updated"`
	UsersSkipped       int `json:"users_skipped"`
	TeamsCreated       int `json:"teams_created"`
	MembershipsAdded   int `json:"memberships_added"`
	MembershipsRemoved int `json:"memberships_removed"`
}

// LDAPSyncService coordinates synchronisation of LDAP users and groups into local teams.
type LDAPSyncService struct {
	db  *gorm.DB
	sso *iauth.SSOManager
}

// NewLDAPSyncService constructs a new LDAP synchronisation helper.
func NewLDAPSyncService(db *gorm.DB, sso *iauth.SSOManager) (*LDAPSyncService, error) {
	if db == nil {
		return nil, errors.New("ldap sync service: db is required")
	}
	if sso == nil {
		return nil, errors.New("ldap sync service: sso manager is required")
	}
	return &LDAPSyncService{db: db, sso: sso}, nil
}

// SyncAll pulls directory identities via the provided authenticator, provisioning accounts and synchronising memberships.
func (s *LDAPSyncService) SyncAll(ctx context.Context, auth *providers.LDAPAuthenticator, cfg models.LDAPConfig, allowProvision bool) (LDAPSyncSummary, error) {
	if auth == nil {
		return LDAPSyncSummary{}, errors.New("ldap sync service: authenticator is required")
	}

	ctx = ensureContext(ctx)

	identities, err := auth.ListIdentities(ctx)
	if err != nil {
		return LDAPSyncSummary{}, fmt.Errorf("ldap sync service: list identities: %w", err)
	}

	values := make([]providers.Identity, 0, len(identities))
	for _, identity := range identities {
		values = append(values, *identity)
	}

	return s.SyncFromIdentities(ctx, cfg, values, allowProvision)
}

// SyncGroups aligns a user's team memberships with the supplied LDAP groups.
func (s *LDAPSyncService) SyncGroups(ctx context.Context, cfg models.LDAPConfig, user *models.User, groups []string) (LDAPUserSyncResult, error) {
	return s.syncGroups(ctx, cfg, user, groups)
}

// SyncFromIdentities processes the supplied identities, provisioning users and synchronising memberships.
func (s *LDAPSyncService) SyncFromIdentities(ctx context.Context, cfg models.LDAPConfig, identities []providers.Identity, allowProvision bool) (LDAPSyncSummary, error) {
	ctx = ensureContext(ctx)

	summary := LDAPSyncSummary{}

	for _, identity := range identities {
		email := strings.TrimSpace(identity.Email)
		if email == "" {
			summary.UsersSkipped++
			continue
		}

		existingLDAPUser, err := s.userExistsForProvider(ctx, identity.Provider, email)
		if err != nil {
			return summary, err
		}

		user, err := s.sso.LinkIdentity(ctx, identity, allowProvision)
		if err != nil {
			switch {
			case errors.Is(err, iauth.ErrSSOUserNotFound):
				summary.UsersSkipped++
				continue
			case errors.Is(err, iauth.ErrSSOEmailRequired):
				summary.UsersSkipped++
				continue
			default:
				return summary, fmt.Errorf("ldap sync service: link identity: %w", err)
			}
		}

		if existingLDAPUser {
			summary.UsersUpdated++
		} else {
			summary.UsersCreated++
		}

		result, err := s.syncGroups(ctx, cfg, user, identity.Groups)
		if err != nil {
			return summary, err
		}

		summary.TeamsCreated += result.TeamsCreated
		summary.MembershipsAdded += result.MembershipsAdded
		summary.MembershipsRemoved += result.MembershipsRemoved
	}

	return summary, nil
}

func (s *LDAPSyncService) userExistsForProvider(ctx context.Context, provider string, email string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&models.User{}).
		Where("LOWER(email) = ?", strings.ToLower(email)).
		Where("auth_provider = ?", strings.ToLower(strings.TrimSpace(provider))).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("ldap sync service: check existing user: %w", err)
	}
	return count > 0, nil
}

func (s *LDAPSyncService) syncGroups(ctx context.Context, cfg models.LDAPConfig, user *models.User, rawGroups []string) (LDAPUserSyncResult, error) {
	result := LDAPUserSyncResult{UserID: user.ID}
	if !cfg.SyncGroups {
		return result, nil
	}
	if cfg.AttributeMapping == nil || strings.TrimSpace(cfg.AttributeMapping["groups"]) == "" {
		return result, nil
	}

	groupMap := normaliseLDAPGroups(rawGroups)

	ctx = ensureContext(ctx)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		currentTeams, err := s.loadCurrentLDAPTeams(tx, user.ID)
		if err != nil {
			return err
		}

		currentByID := make(map[string]models.Team, len(currentTeams))
		for _, team := range currentTeams {
			currentByID[team.ID] = team
		}

		retain := make(map[string]struct{}, len(groupMap))

		for _, group := range groupMap {
			team, created, err := s.findOrCreateLDAPTeam(tx, group)
			if err != nil {
				return err
			}
			if created {
				result.TeamsCreated++
			}
			retain[team.ID] = struct{}{}

			if _, exists := currentByID[team.ID]; !exists {
				if err := tx.Model(team).Association("Users").Append(user); err != nil {
					return fmt.Errorf("ldap sync service: add membership: %w", err)
				}
				result.MembershipsAdded++
			}
		}

		for _, team := range currentTeams {
			if _, ok := retain[team.ID]; ok {
				continue
			}
			if err := tx.Model(&team).Association("Users").Delete(user); err != nil {
				return fmt.Errorf("ldap sync service: remove membership: %w", err)
			}
			result.MembershipsRemoved++
		}

		return nil
	})

	if err != nil {
		return LDAPUserSyncResult{}, err
	}

	return result, nil
}

func (s *LDAPSyncService) loadCurrentLDAPTeams(tx *gorm.DB, userID string) ([]models.Team, error) {
	var teams []models.Team
	if err := tx.
		Table("teams").
		Joins("JOIN user_teams ut ON ut.team_id = teams.id").
		Where("ut.user_id = ?", userID).
		Where("teams.source = ?", "ldap").
		Find(&teams).Error; err != nil {
		return nil, fmt.Errorf("ldap sync service: load current memberships: %w", err)
	}
	return teams, nil
}

type ldapGroup struct {
	ExternalID string
	Name       string
}

func (s *LDAPSyncService) findOrCreateLDAPTeam(tx *gorm.DB, group ldapGroup) (*models.Team, bool, error) {
	if group.ExternalID == "" {
		return nil, false, errors.New("ldap sync service: group external id required")
	}

	var team models.Team
	err := tx.Where("source = ? AND external_id = ?", "ldap", group.ExternalID).First(&team).Error
	switch {
	case err == nil:
		if group.Name != "" && team.Name != group.Name {
			if updateErr := tx.Model(&team).Update("name", group.Name).Error; updateErr != nil {
				return nil, false, fmt.Errorf("ldap sync service: update team name: %w", updateErr)
			}
			if reloadErr := tx.First(&team, "id = ?", team.ID).Error; reloadErr != nil {
				return nil, false, fmt.Errorf("ldap sync service: reload team: %w", reloadErr)
			}
		}
		return &team, false, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		team = models.Team{
			Name:       s.resolveTeamName(group),
			Source:     "ldap",
			ExternalID: group.ExternalID,
		}
		if createErr := tx.Create(&team).Error; createErr != nil {
			return nil, false, fmt.Errorf("ldap sync service: create team: %w", createErr)
		}
		return &team, true, nil
	default:
		return nil, false, fmt.Errorf("ldap sync service: find team: %w", err)
	}
}

func (s *LDAPSyncService) resolveTeamName(group ldapGroup) string {
	name := strings.TrimSpace(group.Name)
	if name != "" {
		return name
	}
	return group.ExternalID
}

func normaliseLDAPGroups(raw []string) map[string]ldapGroup {
	groups := make(map[string]ldapGroup, len(raw))
	for _, value := range raw {
		externalID, name := parseLDAPGroupValue(value)
		if externalID == "" {
			continue
		}
		if existing, ok := groups[externalID]; ok {
			if existing.Name == "" && name != "" {
				existing.Name = name
				groups[externalID] = existing
			}
			continue
		}
		groups[externalID] = ldapGroup{ExternalID: externalID, Name: name}
	}
	return groups
}

func parseLDAPGroupValue(raw string) (string, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}

	name := trimmed
	if strings.Contains(trimmed, "=") && strings.Contains(trimmed, ",") {
		first := strings.Split(trimmed, ",")[0]
		parts := strings.SplitN(first, "=", 2)
		if len(parts) == 2 {
			if candidate := strings.TrimSpace(parts[1]); candidate != "" {
				name = candidate
			}
		}
	}

	externalID := strings.ToLower(trimmed)
	return externalID, name
}
