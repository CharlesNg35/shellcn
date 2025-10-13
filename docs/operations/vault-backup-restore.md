# Vault Backup & Restore Playbook

The credential vault encrypts secrets at rest, but recovering from data loss still
requires a disciplined approach. This playbook documents how to back up, verify, and
restore the vault safely.

## 1. Components to Protect

- **Application database** – contains encrypted identity payloads, shares, and audit logs.
- **Vault master key** – the value of `SHELLCN_VAULT_ENCRYPTION_KEY`. Without this key, the
  encrypted payloads cannot be decrypted.
- **Configuration snapshot** – environment variables, feature flags, and system settings
  that control vault behaviour.

## 2. Backup Cadence

| Asset               | Frequency           | Notes                                                 |
| ------------------- | ------------------- | ----------------------------------------------------- |
| Database            | Hourly + daily full | Use native tooling (pg_dump, mysqldump, SQLite copy). |
| Vault master key    | On change           | Store in a hardware-backed or managed secret store.   |
| Configuration files | On config changes   | Version control with access restrictions.             |

> **Tip:** Include the Prometheus metrics exports in your monitoring snapshots so that
> vault operation counters can be correlated with restores.

## 3. Backup Procedure (Database)

1. Put the control plane into **read-only maintenance** mode if possible.
2. Run the database-specific backup command:

   ```bash
   # PostgreSQL example
   pg_dump --format=custom --file="$(date +%F_%H%M)_shellcn.backup" shellcn
   ```

3. Verify the archive checksum and upload it to long-term storage.
4. Record the backup in your runbook, linking to the matching vault key version.

## 4. Backup Procedure (Vault Key)

1. Retrieve the active value of `SHELLCN_VAULT_ENCRYPTION_KEY` from the secret manager.
2. Encrypt the key material using a secondary key (for example `age`, KMS envelope, or a
   PGP recipient).
3. Store the encrypted blob in an access-controlled location with tamper logging.
4. Update the inventory (key identifier, storage location, retention policy).

## 5. Restore Procedure

1. Identify the backup set (database dump + matching vault key).
2. Provision a new environment with **no external network access** until validation passes.
3. Import the database backup.
4. Restore the vault key as the `SHELLCN_VAULT_ENCRYPTION_KEY` environment variable.
5. Start the application and monitor the startup logs (successful decrypt validation).
6. Run validation checks:
   - Call `GET /api/vault/identities/:id?include=payload` as a root user and verify payload
     access.
   - Confirm `shellcn_vault_operations_total{operation="identity_create",result="success"}`
     increments after creating a test identity.
7. Reconnect downstream services only after validation succeeds.

## 6. Post-Restore Actions

- Rotate any break-glass credentials used during the restore.
- Clear the restored environment of test identities.
- Resume regular backup schedule and monitoring alerts.
- Document the incident in the change log and update this playbook if gaps were found.

## 7. Validation Checklist

- [ ] Database restored and accessible.
- [ ] Vault master key restored and verified.
- [ ] Vault metrics and audit logs active.
- [ ] Test identities created, fetched, and deleted successfully.
- [ ] Monitoring alerts reset and dashboards updated.
