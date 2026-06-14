package plugin

import (
	"fmt"
	"strings"
)

const (
	// CredentialIDField is the conventional config key for a credential_ref field.
	CredentialIDField = "credential_id"
)

// ResolvedCredential is decrypted credential material resolved by the core for
// one credential_ref config field.
type ResolvedCredential struct {
	ID     string
	Kind   CredentialKind
	Values map[string]string
}

// CredentialBinding binds one resolved credential to the config field that
// selected it.
type CredentialBinding struct {
	Field      string
	Credential ResolvedCredential
}

// ResolvedCredentials is the set of reusable credentials resolved for a
// connection attempt.
type ResolvedCredentials struct {
	byField map[string]ResolvedCredential
}

// NewResolvedCredentials builds a resolved credential set keyed by the
// credential_ref field that selected each credential.
func NewResolvedCredentials(bindings ...CredentialBinding) ResolvedCredentials {
	out := ResolvedCredentials{byField: map[string]ResolvedCredential{}}
	for _, binding := range bindings {
		field := strings.TrimSpace(binding.Field)
		if field == "" {
			continue
		}
		out.byField[field] = cloneResolvedCredential(binding.Credential)
	}
	if len(out.byField) == 0 {
		return ResolvedCredentials{}
	}
	return out
}

// For returns the resolved credential for a credential_ref field.
func (r ResolvedCredentials) For(field string) (ResolvedCredential, bool) {
	if r.byField == nil {
		return ResolvedCredential{}, false
	}
	cred, ok := r.byField[field]
	if !ok {
		return ResolvedCredential{}, false
	}
	return cloneResolvedCredential(cred), true
}

// CredentialFor returns the resolved credential for a credential_ref field.
func (c ConnectConfig) CredentialFor(field string) (ResolvedCredential, bool) {
	return c.Credentials.For(field)
}

// RequiredCredentialFor returns a resolved credential or a typed validation
// error when the field was not resolved or resolved to an unexpected kind.
func (c ConnectConfig) RequiredCredentialFor(field string, kind CredentialKind) (ResolvedCredential, error) {
	cred, ok := c.CredentialFor(field)
	if !ok {
		return ResolvedCredential{}, fmt.Errorf("%w: credential field %q is required", ErrInvalidInput, field)
	}
	if kind != "" && cred.Kind != kind {
		return ResolvedCredential{}, fmt.Errorf("%w: credential field %q resolved kind %q, want %q", ErrInvalidInput, field, cred.Kind, kind)
	}
	return cred, nil
}

// CredentialValuesFor returns a copy of the decrypted values for a
// credential_ref field.
func (c ConnectConfig) CredentialValuesFor(field string) map[string]string {
	cred, ok := c.CredentialFor(field)
	if !ok {
		return map[string]string{}
	}
	return cred.Values
}

// CredentialValueFor returns one decrypted credential value.
func (c ConnectConfig) CredentialValueFor(field, key string) string {
	return c.CredentialValuesFor(field)[key]
}

// CredentialKindFor returns the kind resolved for a credential_ref field.
func (c ConnectConfig) CredentialKindFor(field string) CredentialKind {
	cred, ok := c.CredentialFor(field)
	if !ok {
		return ""
	}
	return cred.Kind
}

// Value returns one decrypted credential value.
func (r ResolvedCredential) Value(key string) string {
	return r.Values[key]
}

// RequiredValue returns one decrypted credential value or a typed validation
// error when it is empty.
func (r ResolvedCredential) RequiredValue(key string) (string, error) {
	value := r.Value(key)
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%w: credential field %q is required", ErrInvalidInput, key)
	}
	return value, nil
}

func cloneResolvedCredential(cred ResolvedCredential) ResolvedCredential {
	out := ResolvedCredential{
		ID:     cred.ID,
		Kind:   cred.Kind,
		Values: map[string]string{},
	}
	for k, v := range cred.Values {
		out.Values[k] = v
	}
	return out
}
