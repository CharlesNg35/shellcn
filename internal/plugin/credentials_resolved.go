package plugin

import "strings"

const (
	// CredentialField is the conventional config key for a credential_ref field.
	CredentialField = "credential_id"

	CredentialSecret   = "_credential_secret"
	CredentialIdentity = "_credential_identity"
	CredentialKindKey  = "_credential_kind"
)

func CredentialSecretKey(field string) string {
	if field == CredentialField {
		return CredentialSecret
	}
	return "_" + field + "_secret"
}

func CredentialIdentityKey(field string) string {
	if field == CredentialField {
		return CredentialIdentity
	}
	return "_" + field + "_identity"
}

func CredentialResolvedKindKey(field string) string {
	if field == CredentialField {
		return CredentialKindKey
	}
	return "_" + field + "_kind"
}

func (c ConnectConfig) CredentialSecretFor(field string) string {
	return c.String(CredentialSecretKey(field))
}

func (c ConnectConfig) CredentialIdentityFor(field string) string {
	return strings.TrimSpace(c.String(CredentialIdentityKey(field)))
}

func (c ConnectConfig) CredentialKindFor(field string) CredentialKind {
	return CredentialKind(strings.TrimSpace(c.String(CredentialResolvedKindKey(field))))
}
