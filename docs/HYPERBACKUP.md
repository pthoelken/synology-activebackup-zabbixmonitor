# Hyper Backup

Implements [feature request #2](https://github.com/pthoelken/synology-activebackup-zabbixmonitor/issues/2) as the product `hyperbackup` alongside `abb` and `m365`.

## Enable and upgrade

1. Install the 0.2.6 SPK matching the NAS architecture.
2. Re-import the updated API or sender Zabbix template. Preserve the host's existing macros. Template names, UUIDs and existing item keys remain unchanged.
3. In the DSM app, select **Config → Products → Hyper Backup**. Enter the DSM HTTPS URL, username and password (recommended for the standard package user), save and restart the package through Package Center. Alternatively set:

   ```yaml
   products:
     hyper_backup:
       enabled: true
       api_url: "https://nas.example.com:5001"
       username: "backup-monitor"
       password: "replace-me"
   ```

4. Inspect **Sources**, **Jobs** and collector errors, then run discovery in Zabbix (or wait for its next execution). Sender mode may reject new job values until discovery has created the corresponding items, as with ABB/M365.

The default is disabled so installations without Hyper Backup keep their current behavior. The feature needs the Hyper Backup package and a DSM account authorized to read Hyper Backup tasks. HTTPS is supported without elevating the package user. No external runtime is required.

## Source and permissions

With `api_url` configured, the collector signs in using `SYNO.API.Auth` v6 at `/webapi/auth.cgi`, reads the Hyper Backup task list and status through `/webapi/entry.cgi` with the returned SID, and logs out through `/webapi/auth.cgi` after collection. It does not supply an application session name; access is governed by the DSM account and delegated Hyper Backup role. Use the NAS hostname matching its trusted TLS certificate, for example `https://nas.example.com:5001`. Only HTTPS is accepted; certificate/hostname verification is enabled by default and redirects are rejected. The NAS must trust the certificate issuer. Credentials and session IDs are sent in POST bodies and are excluded from collector errors. Use a DSM account with the minimum permissions needed for Hyper Backup. Accounts requiring interactive 2FA cannot use this unattended login; respect the NAS account policy rather than disabling 2FA on a personal administrator account.

The password is masked in the DSM form and stored in the package configuration, just like the existing API token and Zabbix PSK. Configuration writes now enforce mode `0600`. The daemon and DSM CGI must run with access to the package-owned configuration. Authenticated configuration API clients can read these secrets, so protect the existing monitoring API token and its network access.

With an empty `api_url`, the collector uses local access. The collector runs `/usr/syno/bin/synowebapi --exec api=SYNO.Backup.Task version=1 method=list` and then `method=status` per task, requesting last-backup metadata with `blOnline=false`. It does not initiate backups, restore, rotate versions, or alter task configuration.

These are internal Synology APIs; compatibility and permissions can vary by DSM/Hyper Backup version. In particular, successful execution from a root SSH session does not prove the package account can use the API. If the package account is denied, the collector reports the DSM error code, health becomes unhealthy, and the Hyper Backup source alarm fires. Enabling the checkbox does not grant DSM permissions. Use the HTTPS mode if local access is denied. Do not change the entire monitoring daemon to root as a workaround.

The call structure is based on the [synology-api implementation](https://github.com/N4S4/synology-api/blob/master/synology_api/core_backup.py); timestamp fields/formats are also consumed by [synology_backup_exporter](https://github.com/raph2i/synology_backup_exporter/blob/main/init.py). [This Zabbix integration](https://github.com/lestoilfante/zabbix-integrations/blob/master/Synology/template_synology_hyperbackup.yaml) records the textual task result/state values. No third-party implementation is bundled. Tests use synthetic API fixtures matching these observed schemas, not captured NAS responses.

## Data semantics

- Stable task ID, name and target type are included in discovery, with `product=hyperbackup`.
- `done`/`success` → OK; partial/cancelled/suspended/discarded → Warning; backing up/resuming/version deletion → Running; failure/checksum failure/missing destination → Failed. Unrecognized result, activity and repository state values remain Unknown. Exportable/importable/relinkable repositories produce Warning. Current activity takes precedence over the previous result; broken, unauthorized, retired and restore-only repositories are failures.
- Empty/`none` results are No data. Successful results without an end timestamp are also No data. New tasks remain discoverable, so the existing no-data trigger applies.
- Start, end and last-success timestamps accept Unix values and NAS-local date strings (with or without seconds). Missing timestamps stay absent. The collector never substitutes the current time for a missing backup. An explicitly successful last result may use its end time as the last-success time if that field is absent.
- Runtime and age describe the metadata returned by Hyper Backup, which may still refer to the previous run while a new backup is active. Current activity is available separately in `info.activity`.
- `SYNO.Backup.Task.status` does not return a completed-run byte count. Discovery therefore marks transferred size as unavailable for Hyper Backup, and sender mode does not submit a fabricated zero. The Zabbix templates suppress this item for Hyper Backup while retaining it for ABB and Microsoft 365.
- Query/schema errors preserve discovered tasks as Unknown and expose a collector error. An empty task list is valid. The legacy health field `db_missing` means unavailable data source for Hyper Backup, including API failures.

## Verify on a NAS

After enabling and restarting, inspect:

```sh
synology-activebackup-zabbix status --fresh
synology-activebackup-zabbix discovery --product hyperbackup
synology-activebackup-zabbix job --product hyperbackup --task-id 1 --field info
```

The same data is available using the existing authenticated API:

```text
GET /api/v1/discovery?product=hyperbackup
GET /api/v1/job?product=hyperbackup&task_id=1&field=info
GET /api/v1/health?field=db_missing&product=hyperbackup
```

Compare a successful, failed, running and never-run task against the Hyper Backup UI. Check last-success time after a failed backup and check the NAS timezone. Verify that disabling the product clears its source alarm and that ABB/M365 still collect normally. For compatibility reports, include DSM and Hyper Backup versions, the numeric API error (if any), and anonymized list/status JSON; remove names and destination/account details.

Automated tests and cross-architecture builds cannot establish permissions or schema compatibility on a real DSM. A NAS acceptance test remains necessary before production rollout.

## HTTPS connection errors (0.2.1 and later)

Transport failures now distinguish an untrusted certificate issuer, hostname/IP mismatch, expired/not-yet-valid certificate, DNS failure, timeout, refused connection and invalid TLS response. Diagnostic messages omit raw URLs, certificate identities and credentials. Certificate verification remains enabled unless explicitly disabled using the option below.

Use the DSM HTTPS hostname covered by its certificate. That hostname must resolve and be reachable from the NAS itself. A browser trusting a private CA does not imply the NAS trusts it. When the issuer is untrusted, configure a DSM certificate with a trusted issuer and its full certificate chain. For a refused connection or invalid TLS response, verify the DSM HTTPS port (commonly 5001); the monitor's port 9876 is a separate service.

## Ignore certificate errors (0.2.2 and later)

Under **Config → Products**, enable **Ignore Hyper Backup TLS certificate errors (server identity is not verified)**, save and restart the package. Alternatively set `products.hyper_backup.insecure_skip_verify: true` in the configuration. The default is `false`, including for existing configurations that omit the setting.

This option skips certificate chain, validity and hostname/IP verification for Hyper Backup HTTPS requests only. Traffic remains encrypted, but the server's identity is not authenticated. Use it only when you accept that risk. It does not change Zabbix TLS, allow HTTP URLs, or fix DNS, connectivity or login errors. Disable the option and restart to restore normal certificate verification.
