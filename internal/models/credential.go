package models

import "time"

// Credential is a reusable encrypted secret bundle with its own ownership and
// grants, referenced by many connections without exposing its value.
type Credential struct {
	ID        string `gorm:"primaryKey"`
	Name      string
	Kind      string   `gorm:"index"` // ssh_private_key, ssh_password, tls_client_cert, db_password, api_token, …
	OwnerID   string   `gorm:"index"`
	Username  string   // optional identity/principal metadata; column name kept for existing databases
	Protocols []string `gorm:"serializer:json"` // allowed protocols; empty = any compatible
	// EncryptedSecret is opaque ciphertext; the store never sees plaintext.
	EncryptedSecret []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (Credential) TableName() string { return "credentials" }

// CredentialSummary is the non-secret view returned to clients for selection.
// It never carries secret material, encrypted blobs, or storage keys.
type CredentialSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	OwnerID   string    `json:"ownerId,omitempty"`
	Identity  string    `json:"identity,omitempty"`
	Protocols []string  `json:"protocols,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitzero"`
}

// Summary projects a Credential to its non-secret summary.
func (c Credential) Summary() CredentialSummary {
	return CredentialSummary{
		ID:        c.ID,
		Name:      c.Name,
		Kind:      c.Kind,
		OwnerID:   c.OwnerID,
		Identity:  c.Username,
		Protocols: c.Protocols,
		UpdatedAt: c.UpdatedAt,
	}
}

// CredentialGrant shares a credential's use with a subject without readback.
type CredentialGrant struct {
	ID           string `gorm:"primaryKey"`
	CredentialID string `gorm:"index;uniqueIndex:idx_credgrant_cred_subject"`
	SubjectID    string `gorm:"index;uniqueIndex:idx_credgrant_cred_subject"`
	Access       Access // typically AccessUse
	CreatedAt    time.Time
}

func (CredentialGrant) TableName() string { return "credential_grants" }
