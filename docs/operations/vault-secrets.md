# Vault Encryption Key Operations Guide

The ShellCN vault encrypts all credential payloads with a master key derived from the
`SHELLCN_VAULT_ENCRYPTION_KEY` environment variable. This document summarises how to
generate, store, and rotate that key safely.

## 1. Key Generation

- **Length**: provide 32 bytes of entropy before encoding. The runtime derives a
  32-byte Argon2id key for AES-256-GCM encryption.
- **Encoding**: hex-encoded values are recommended, but base64 and raw strings are
  accepted. The configuration loader trims whitespace automatically.
- **Example (Linux/macOS)**:

  ```bash
  head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
  ```

  Export the value as `SHELLCN_VAULT_ENCRYPTION_KEY` in the service environment.

## 2. Storage Recommendations

- Store the plaintext key in a managed secret store (AWS Secrets Manager, GCP Secret
  Manager, HashiCorp Vault) or platform-specific secret tooling (Kubernetes Secrets,
  systemd drop-in files with strict permissions).
- Restrict access to platform administrators. The key grants the ability to decrypt
  every credential stored in the vault.
- Avoid committing the key to configuration files, Terraform state, or CI variables that
  are shared broadly.
- During containerised deployments, mount the secret via environment variables or a
  file volume that is readable only by the application user.

## 3. Verification Checklist

- Application starts without `vault.encryption_key` validation errors.
- `VaultKeyMetadata` table contains an active row for the primary key after the first
  launch (`shellcn server` persists metadata automatically).
- A smoke test encrypt/decrypt call succeeds (for example, create a test identity
  through the API and retrieve it).
- Audit logs confirm `vault.identity.created` entries with masked payloads.

## 4. Rotation Guidance

- Rotation is _not_ automated in the current release. Plan rotations during a
  maintenance window.
- Procedure:
  1. Generate a new key and update the environment variable.
  2. Restart the application.
  3. Update the stored system setting via `EnsureVaultEncryptionKey` helper or through
     the administration API (keeps the UI in sync).
  4. Validate that new identities can be created and existing ones decrypted.
- Document every rotation event in your change management system and store previous keys
  in a break-glass location until all credentials are re-encrypted.

## 5. Disaster Recovery

- Back up the application database on a regular cadence; encrypted identities remain
  unusable without the master key.
- Keep an offline copy of the current vault key (encrypted at rest). Without the key,
  identity records are unrecoverable.
- During recovery: restore the database, restore the key to the environment, restart the
  application, and verify identity access.

## 6. Security Checklist

- [ ] `SHELLCN_VAULT_ENCRYPTION_KEY` has 32 bytes of entropy.
- [ ] Key is stored in a managed secret system with access logging.
- [ ] Runtime hosts restrict environment inspection to privileged operators.
- [ ] Rotation and recovery procedures are documented and tested.
