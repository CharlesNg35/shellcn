package secrets

import "context"

// Placeholder replaces a secret value anywhere it would otherwise be shown.
const Placeholder = "***"

// State reports a write-only secret's presence without revealing it.
func State(present bool) string {
	if present {
		return "set"
	}
	return "not set"
}

// RedactParams returns a copy of params with the values of secretKeys replaced
// by Placeholder — used before params reach logs or the audit log.
func RedactParams(params map[string]string, secretKeys map[string]bool) map[string]string {
	if len(params) == 0 {
		return nil
	}
	out := make(map[string]string, len(params))
	for k, v := range params {
		if secretKeys[k] {
			out[k] = Placeholder
		} else {
			out[k] = v
		}
	}
	return out
}

// DecryptMap decrypts every ciphertext value into its plaintext string. Used at
// connect time to turn a connection's stored inline secrets into usable config.
func DecryptMap(ctx context.Context, store SecretStore, in map[string][]byte) (map[string]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(in))
	for k, blob := range in {
		pt, err := store.Decrypt(ctx, blob)
		if err != nil {
			return nil, err
		}
		out[k] = string(pt)
	}
	return out, nil
}

// EncryptMap encrypts every plaintext value into ciphertext for storage.
func EncryptMap(ctx context.Context, store SecretStore, in map[string]string) (map[string][]byte, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		ct, err := store.Encrypt(ctx, []byte(v))
		if err != nil {
			return nil, err
		}
		out[k] = ct
	}
	return out, nil
}
