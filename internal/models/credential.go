package models

import "time"

// Credential is a reusable encrypted secret bundle with its own ownership and
// grants, referenced by many connections without exposing its value.
type Credential struct {
	ID        string            `gorm:"primaryKey"`
	Name      string            `gorm:"not null"`
	Kind      string            `gorm:"index;not null"`
	OwnerID   string            `gorm:"index;not null"`
	Values    map[string]string `gorm:"serializer:json"`
	Protocols []string          `gorm:"serializer:json"`
	// EncryptedValues is encrypted JSON for secret credential fields.
	EncryptedValues []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (Credential) TableName() string { return "credentials" }

// CredentialSummary is the non-secret view returned to clients for selection.
// It never carries secret material, encrypted blobs, or storage keys.
type CredentialSummary struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	OwnerID   string            `json:"ownerId,omitempty"`
	OwnerName string            `json:"ownerName,omitempty"`
	Values    map[string]string `json:"values,omitempty"`
	Protocols []string          `json:"protocols,omitempty"`
	UpdatedAt time.Time         `json:"updatedAt,omitzero"`
}

// Summary projects a Credential to its non-secret summary.
func (c Credential) Summary() CredentialSummary {
	values := make(map[string]string, len(c.Values))
	for k, v := range c.Values {
		values[k] = v
	}
	return CredentialSummary{
		ID:        c.ID,
		Name:      c.Name,
		Kind:      c.Kind,
		OwnerID:   c.OwnerID,
		Values:    values,
		Protocols: c.Protocols,
		UpdatedAt: c.UpdatedAt,
	}
}

// CredentialGrant shares a credential's use with a subject without readback.
type CredentialGrant struct {
	ID           string `gorm:"primaryKey"`
	CredentialID string `gorm:"index;uniqueIndex:idx_credgrant_cred_subject"`
	SubjectID    string `gorm:"index;uniqueIndex:idx_credgrant_cred_subject"`
	Access       Access // typically AccessView
	CreatedAt    time.Time
}

func (CredentialGrant) TableName() string { return "credential_grants" }
