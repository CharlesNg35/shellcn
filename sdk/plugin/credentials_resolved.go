package plugin

import "strings"

const (
	// CredentialIDField is the conventional config key for a credential_ref field.
	CredentialIDField = "credential_id"

	CredentialIDValuesKey = "_credential_values"
	CredentialIDKindKey   = "_credential_kind"
)

func CredentialValuesKey(field string) string {
	if field == CredentialIDField {
		return CredentialIDValuesKey
	}
	return "_" + field + "_values"
}

func CredentialResolvedKindKey(field string) string {
	if field == CredentialIDField {
		return CredentialIDKindKey
	}
	return "_" + field + "_kind"
}

func (c ConnectConfig) CredentialValuesFor(field string) map[string]string {
	raw := c.Config[CredentialValuesKey(field)]
	switch values := raw.(type) {
	case map[string]string:
		out := make(map[string]string, len(values))
		for k, v := range values {
			out[k] = v
		}
		return out
	case map[string]any:
		out := make(map[string]string, len(values))
		for k, v := range values {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
		return out
	default:
		return map[string]string{}
	}
}

func (c ConnectConfig) CredentialValueFor(field, key string) string {
	return c.CredentialValuesFor(field)[key]
}

func (c ConnectConfig) CredentialKindFor(field string) CredentialKind {
	return CredentialKind(strings.TrimSpace(c.String(CredentialResolvedKindKey(field))))
}
