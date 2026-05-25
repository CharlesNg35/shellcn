package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/charlesng/shellcn/internal/models"
)

// normNotFound maps GORM's record-not-found to the store sentinel.
func normNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// rowsOrNotFound returns ErrNotFound when an update/delete matched no row.
func rowsOrNotFound(res *gorm.DB) error {
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

type gormUserStore struct{ db *gorm.DB }

func (s *gormUserStore) Create(ctx context.Context, u *models.User, passwordHash string) error {
	row := *u
	row.PasswordHash = passwordHash
	return s.db.WithContext(ctx).Create(&row).Error
}

func (s *gormUserStore) GetByID(ctx context.Context, id string) (models.User, error) {
	var u models.User
	if err := s.db.WithContext(ctx).First(&u, "id = ?", id).Error; err != nil {
		return models.User{}, normNotFound(err)
	}
	u.PasswordHash = ""
	return u, nil
}

func (s *gormUserStore) GetByUsername(ctx context.Context, username string) (models.User, error) {
	var u models.User
	if err := s.db.WithContext(ctx).First(&u, "username = ?", username).Error; err != nil {
		return models.User{}, normNotFound(err)
	}
	u.PasswordHash = ""
	return u, nil
}

func (s *gormUserStore) GetPasswordHash(ctx context.Context, userID string) (string, error) {
	var u models.User
	if err := s.db.WithContext(ctx).Select("password_hash").First(&u, "id = ?", userID).Error; err != nil {
		return "", normNotFound(err)
	}
	return u.PasswordHash, nil
}

func (s *gormUserStore) SetPasswordHash(ctx context.Context, userID, hash string) error {
	res := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("password_hash", hash)
	return rowsOrNotFound(res)
}

func (s *gormUserStore) List(ctx context.Context) ([]models.User, error) {
	var users []models.User
	if err := s.db.WithContext(ctx).Omit("password_hash").Order("username").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (s *gormUserStore) Update(ctx context.Context, u *models.User) error {
	res := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", u.ID).
		Select("username", "email", "display_name", "roles", "disabled").Updates(u)
	return rowsOrNotFound(res)
}

func (s *gormUserStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.User{}, "id = ?", id).Error
}

func (s *gormUserStore) Count(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&models.User{}).Count(&n).Error
	return n, err
}

type gormConnectionStore struct{ db *gorm.DB }

func (s *gormConnectionStore) Create(ctx context.Context, c *models.Connection) error {
	return s.db.WithContext(ctx).Create(c).Error
}

func (s *gormConnectionStore) Get(ctx context.Context, id string) (models.Connection, error) {
	var c models.Connection
	if err := s.db.WithContext(ctx).First(&c, "id = ?", id).Error; err != nil {
		return models.Connection{}, normNotFound(err)
	}
	return c, nil
}

func (s *gormConnectionStore) ListByOwner(ctx context.Context, ownerID string) ([]models.Connection, error) {
	var list []models.Connection
	if err := s.db.WithContext(ctx).Where("owner_id = ?", ownerID).Order("name").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *gormConnectionStore) List(ctx context.Context) ([]models.Connection, error) {
	var list []models.Connection
	if err := s.db.WithContext(ctx).Order("name").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *gormConnectionStore) Update(ctx context.Context, c *models.Connection) error {
	res := s.db.WithContext(ctx).Model(&models.Connection{}).Where("id = ?", c.ID).
		Select("name", "protocol", "transport", "shared", "config", "secrets").Updates(c)
	return rowsOrNotFound(res)
}

func (s *gormConnectionStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Connection{}, "id = ?", id).Error
}

type gormCredentialStore struct{ db *gorm.DB }

func (s *gormCredentialStore) Create(ctx context.Context, c *models.Credential) error {
	return s.db.WithContext(ctx).Create(c).Error
}

func (s *gormCredentialStore) Get(ctx context.Context, id string) (models.Credential, error) {
	var c models.Credential
	if err := s.db.WithContext(ctx).First(&c, "id = ?", id).Error; err != nil {
		return models.Credential{}, normNotFound(err)
	}
	return c, nil
}

func (s *gormCredentialStore) ListByOwner(ctx context.Context, ownerID string) ([]models.Credential, error) {
	var list []models.Credential
	if err := s.db.WithContext(ctx).Where("owner_id = ?", ownerID).Order("name").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *gormCredentialStore) Update(ctx context.Context, c *models.Credential) error {
	res := s.db.WithContext(ctx).Model(&models.Credential{}).Where("id = ?", c.ID).
		Select("name", "kind", "username", "protocols", "encrypted_secret").Updates(c)
	return rowsOrNotFound(res)
}

func (s *gormCredentialStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Credential{}, "id = ?", id).Error
}

type gormGrantStore struct{ db *gorm.DB }

func (s *gormGrantStore) Create(ctx context.Context, g *models.Grant) error {
	return s.db.WithContext(ctx).Create(g).Error
}

func (s *gormGrantStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Grant{}, "id = ?", id).Error
}

func (s *gormGrantStore) Get(ctx context.Context, connectionID, subjectID string) (models.Grant, error) {
	var g models.Grant
	if err := s.db.WithContext(ctx).First(&g, "connection_id = ? AND subject_id = ?", connectionID, subjectID).Error; err != nil {
		return models.Grant{}, normNotFound(err)
	}
	return g, nil
}

func (s *gormGrantStore) ListByConnection(ctx context.Context, connectionID string) ([]models.Grant, error) {
	var list []models.Grant
	if err := s.db.WithContext(ctx).Where("connection_id = ?", connectionID).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *gormGrantStore) ListBySubject(ctx context.Context, subjectID string) ([]models.Grant, error) {
	var list []models.Grant
	if err := s.db.WithContext(ctx).Where("subject_id = ?", subjectID).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

type gormCredentialGrantStore struct{ db *gorm.DB }

func (s *gormCredentialGrantStore) Create(ctx context.Context, g *models.CredentialGrant) error {
	return s.db.WithContext(ctx).Create(g).Error
}

func (s *gormCredentialGrantStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.CredentialGrant{}, "id = ?", id).Error
}

func (s *gormCredentialGrantStore) Has(ctx context.Context, credentialID, subjectID string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&models.CredentialGrant{}).
		Where("credential_id = ? AND subject_id = ?", credentialID, subjectID).Count(&n).Error
	return n > 0, err
}

func (s *gormCredentialGrantStore) ListByCredential(ctx context.Context, credentialID string) ([]models.CredentialGrant, error) {
	var list []models.CredentialGrant
	if err := s.db.WithContext(ctx).Where("credential_id = ?", credentialID).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *gormCredentialGrantStore) ListBySubject(ctx context.Context, subjectID string) ([]models.CredentialGrant, error) {
	var list []models.CredentialGrant
	if err := s.db.WithContext(ctx).Where("subject_id = ?", subjectID).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

type gormAuditStore struct{ db *gorm.DB }

func (s *gormAuditStore) Append(ctx context.Context, e *models.AuditEntry) error {
	return s.db.WithContext(ctx).Create(e).Error
}

func (s *gormAuditStore) List(ctx context.Context, f AuditFilter) ([]models.AuditEntry, error) {
	q := s.db.WithContext(ctx).Model(&models.AuditEntry{}).Order("time DESC")
	if f.UserID != "" {
		q = q.Where("user_id = ?", f.UserID)
	}
	if f.ConnectionID != "" {
		q = q.Where("connection_id = ?", f.ConnectionID)
	}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	var list []models.AuditEntry
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

type gormSnippetStore struct{ db *gorm.DB }

type gormPolicyStore struct{ db *gorm.DB }

func (s *gormPolicyStore) Create(ctx context.Context, p *models.PolicyRule) error {
	return s.db.WithContext(ctx).Create(p).Error
}

func (s *gormPolicyStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.PolicyRule{}, "id = ?", id).Error
}

func (s *gormPolicyStore) List(ctx context.Context) ([]models.PolicyRule, error) {
	var list []models.PolicyRule
	if err := s.db.WithContext(ctx).Order("created_at ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *gormSnippetStore) Create(ctx context.Context, sn *models.Snippet) error {
	return s.db.WithContext(ctx).Create(sn).Error
}

func (s *gormSnippetStore) Get(ctx context.Context, id string) (models.Snippet, error) {
	var sn models.Snippet
	if err := s.db.WithContext(ctx).First(&sn, "id = ?", id).Error; err != nil {
		return models.Snippet{}, normNotFound(err)
	}
	return sn, nil
}

func (s *gormSnippetStore) ListByOwner(ctx context.Context, ownerID, protocol string) ([]models.Snippet, error) {
	q := s.db.WithContext(ctx).Where("owner_id = ?", ownerID)
	if protocol != "" {
		q = q.Where("protocol = ?", protocol)
	}
	var list []models.Snippet
	if err := q.Order("name").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *gormSnippetStore) Update(ctx context.Context, sn *models.Snippet) error {
	res := s.db.WithContext(ctx).Model(&models.Snippet{}).Where("id = ?", sn.ID).
		Select("name", "body", "protocol").Updates(sn)
	return rowsOrNotFound(res)
}

func (s *gormSnippetStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Snippet{}, "id = ?", id).Error
}

type gormPreferenceStore struct{ db *gorm.DB }

func (s *gormPreferenceStore) Get(ctx context.Context, userID, key string) (models.Preference, error) {
	var p models.Preference
	if err := s.db.WithContext(ctx).First(&p, "user_id = ? AND pref_key = ?", userID, key).Error; err != nil {
		return models.Preference{}, normNotFound(err)
	}
	return p, nil
}

func (s *gormPreferenceStore) Set(ctx context.Context, p *models.Preference) error {
	p.UpdatedAt = time.Now()
	return s.db.WithContext(ctx).Save(p).Error
}

func (s *gormPreferenceStore) Delete(ctx context.Context, userID, key string) error {
	return s.db.WithContext(ctx).Delete(&models.Preference{}, "user_id = ? AND pref_key = ?", userID, key).Error
}

type gormEnrollmentStore struct{ db *gorm.DB }

func (s *gormEnrollmentStore) Create(ctx context.Context, e *models.AgentEnrollment) error {
	return s.db.WithContext(ctx).Create(e).Error
}

func (s *gormEnrollmentStore) Get(ctx context.Context, id string) (models.AgentEnrollment, error) {
	var e models.AgentEnrollment
	if err := s.db.WithContext(ctx).First(&e, "id = ?", id).Error; err != nil {
		return models.AgentEnrollment{}, normNotFound(err)
	}
	return e, nil
}

func (s *gormEnrollmentStore) GetByTokenHash(ctx context.Context, tokenHash string) (models.AgentEnrollment, error) {
	var e models.AgentEnrollment
	if err := s.db.WithContext(ctx).First(&e, "token_hash = ?", tokenHash).Error; err != nil {
		return models.AgentEnrollment{}, normNotFound(err)
	}
	return e, nil
}

func (s *gormEnrollmentStore) ListByConnection(ctx context.Context, connectionID string) ([]models.AgentEnrollment, error) {
	var list []models.AgentEnrollment
	if err := s.db.WithContext(ctx).Where("connection_id = ?", connectionID).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *gormEnrollmentStore) UpdateStatus(ctx context.Context, id string, status models.AgentEnrollmentStatus) error {
	res := s.db.WithContext(ctx).Model(&models.AgentEnrollment{}).Where("id = ?", id).
		Update("status", string(status))
	return rowsOrNotFound(res)
}
