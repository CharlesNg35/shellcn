package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/charlesng35/shellcn/internal/models"
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
	res := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"password_hash":   hash,
		"session_version": gorm.Expr("COALESCE(session_version, 0) + ?", 1),
	})
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
		Select("username", "email", "display_name", "roles", "disabled", "session_version").Updates(u)
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
		Select("name", "protocol", "transport", "shared", "config", "secrets", "recording", "retention_days").Updates(c)
	return rowsOrNotFound(res)
}

func (s *gormConnectionStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Connection{}, "id = ?", id).Error
}

type gormConnectionFolderStore struct{ db *gorm.DB }

func (s *gormConnectionFolderStore) Create(ctx context.Context, f *models.ConnectionFolder) error {
	return s.db.WithContext(ctx).Create(f).Error
}

func (s *gormConnectionFolderStore) Get(ctx context.Context, id string) (models.ConnectionFolder, error) {
	var f models.ConnectionFolder
	if err := s.db.WithContext(ctx).First(&f, "id = ?", id).Error; err != nil {
		return models.ConnectionFolder{}, normNotFound(err)
	}
	return f, nil
}

func (s *gormConnectionFolderStore) ListByUser(ctx context.Context, userID string) ([]models.ConnectionFolder, error) {
	var list []models.ConnectionFolder
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("sort_order, name").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *gormConnectionFolderStore) Update(ctx context.Context, f *models.ConnectionFolder) error {
	res := s.db.WithContext(ctx).Model(&models.ConnectionFolder{}).Where("id = ?", f.ID).
		Select("parent_id", "name", "color", "sort_order", "updated_at").Updates(f)
	return rowsOrNotFound(res)
}

func (s *gormConnectionFolderStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.ConnectionFolder{}, "id = ?", id).Error
}

type gormConnectionPlacementStore struct{ db *gorm.DB }

func (s *gormConnectionPlacementStore) ListByUser(ctx context.Context, userID string) ([]models.ConnectionPlacement, error) {
	var list []models.ConnectionPlacement
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *gormConnectionPlacementStore) Set(ctx context.Context, p *models.ConnectionPlacement) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "connection_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"folder_id", "sort_order", "updated_at"}),
	}).Create(p).Error
}

func (s *gormConnectionPlacementStore) Delete(ctx context.Context, userID, connectionID string) error {
	return s.db.WithContext(ctx).Delete(&models.ConnectionPlacement{}, "user_id = ? AND connection_id = ?", userID, connectionID).Error
}

func (s *gormConnectionPlacementStore) DeleteByConnection(ctx context.Context, connectionID string) error {
	return s.db.WithContext(ctx).Delete(&models.ConnectionPlacement{}, "connection_id = ?", connectionID).Error
}

func (s *gormConnectionPlacementStore) ClearFolder(ctx context.Context, userID, folderID string) error {
	return s.MoveFolder(ctx, userID, folderID, "")
}

func (s *gormConnectionPlacementStore) MoveFolder(ctx context.Context, userID, folderID, targetFolderID string) error {
	return s.db.WithContext(ctx).Model(&models.ConnectionPlacement{}).
		Where("user_id = ? AND folder_id = ?", userID, folderID).
		Updates(map[string]any{"folder_id": targetFolderID, "updated_at": time.Now()}).Error
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

type gormInvitationStore struct{ db *gorm.DB }

func (s *gormInvitationStore) Create(ctx context.Context, i *models.Invitation) error {
	return s.db.WithContext(ctx).Create(i).Error
}

func (s *gormInvitationStore) Get(ctx context.Context, id string) (models.Invitation, error) {
	var i models.Invitation
	if err := s.db.WithContext(ctx).First(&i, "id = ?", id).Error; err != nil {
		return models.Invitation{}, normNotFound(err)
	}
	return i, nil
}

func (s *gormInvitationStore) GetByTokenHash(ctx context.Context, tokenHash string) (models.Invitation, error) {
	var i models.Invitation
	if err := s.db.WithContext(ctx).First(&i, "token_hash = ?", tokenHash).Error; err != nil {
		return models.Invitation{}, normNotFound(err)
	}
	return i, nil
}

func (s *gormInvitationStore) List(ctx context.Context) ([]models.Invitation, error) {
	var list []models.Invitation
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *gormInvitationStore) Update(ctx context.Context, i *models.Invitation) error {
	res := s.db.WithContext(ctx).Model(&models.Invitation{}).Where("id = ?", i.ID).
		Select("status", "accepted_at").Updates(i)
	return rowsOrNotFound(res)
}

func (s *gormInvitationStore) Consume(ctx context.Context, id string, acceptedAt time.Time) (bool, error) {
	res := s.db.WithContext(ctx).Model(&models.Invitation{}).
		Where("id = ? AND status = ? AND expires_at > ?", id, string(models.InvitePending), acceptedAt).
		Updates(map[string]any{"status": string(models.InviteAccepted), "accepted_at": acceptedAt})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (s *gormInvitationStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Invitation{}, "id = ?", id).Error
}

type gormRecordingStore struct{ db *gorm.DB }

func (s *gormRecordingStore) Create(ctx context.Context, r *models.Recording) error {
	return s.db.WithContext(ctx).Create(r).Error
}

func (s *gormRecordingStore) Get(ctx context.Context, id string) (models.Recording, error) {
	var r models.Recording
	if err := s.db.WithContext(ctx).First(&r, "id = ?", id).Error; err != nil {
		return models.Recording{}, normNotFound(err)
	}
	return r, nil
}

func (s *gormRecordingStore) Update(ctx context.Context, r *models.Recording) error {
	// A map (not a struct) guarantees every column is written — including the
	// nullable *time.Time fields back to NULL — matching the memory store.
	res := s.db.WithContext(ctx).Model(&models.Recording{}).Where("id = ?", r.ID).
		Updates(map[string]any{
			"status":      r.Status,
			"title":       r.Title,
			"ended_at":    r.EndedAt,
			"duration_ms": r.DurationMS,
			"size":        r.Size,
			"checksum":    r.Checksum,
			"storage_key": r.StorageKey,
			"error":       r.Error,
			"expires_at":  r.ExpiresAt,
		})
	return rowsOrNotFound(res)
}

func (s *gormRecordingStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Recording{}, "id = ?", id).Error
}

func (s *gormRecordingStore) CountByUser(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		UserID string
		N      int64
	}
	if err := s.db.WithContext(ctx).Model(&models.Recording{}).
		Select("user_id, count(*) as n").Group("user_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.UserID] = r.N
	}
	return out, nil
}

func (s *gormRecordingStore) List(ctx context.Context, f RecordingFilter) ([]models.Recording, error) {
	q := s.db.WithContext(ctx).Model(&models.Recording{}).Order("started_at DESC")
	if f.UserID != "" {
		q = q.Where("user_id = ?", f.UserID)
	}
	if f.ConnectionID != "" {
		q = q.Where("connection_id = ?", f.ConnectionID)
	}
	if f.Protocol != "" {
		q = q.Where("protocol = ?", f.Protocol)
	}
	if f.Class != "" {
		q = q.Where("class = ?", f.Class)
	}
	if f.Format != "" {
		q = q.Where("format = ?", f.Format)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if !f.Since.IsZero() {
		q = q.Where("started_at >= ?", f.Since)
	}
	if !f.Until.IsZero() {
		q = q.Where("started_at <= ?", f.Until)
	}
	if !f.ExpiredBefore.IsZero() {
		q = q.Where("expires_at IS NOT NULL AND expires_at <= ?", f.ExpiredBefore)
	}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	var list []models.Recording
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
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

func (s *gormEnrollmentStore) UpdateToken(ctx context.Context, id, tokenHash string, expiresAt time.Time) error {
	res := s.db.WithContext(ctx).Model(&models.AgentEnrollment{}).Where("id = ?", id).
		Updates(map[string]any{"token_hash": tokenHash, "expires_at": expiresAt, "updated_at": time.Now()})
	return rowsOrNotFound(res)
}

func (s *gormEnrollmentStore) Consume(ctx context.Context, id string, now time.Time) (bool, error) {
	res := s.db.WithContext(ctx).Model(&models.AgentEnrollment{}).
		Where("id = ? AND (status IN ? OR (status = ? AND expires_at > ?))",
			id,
			[]string{string(models.EnrollmentOffline), string(models.EnrollmentOnline)},
			string(models.EnrollmentPending),
			now,
		).
		Updates(map[string]any{"status": string(models.EnrollmentOnline), "updated_at": now})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}
