# Configuration

Default path:

```text
/var/packages/synology-activebackup-zabbix/etc/config.yml
```

Default config:

```yaml
collector:
  interval_seconds: 300
  max_age_hours: 30

api:
  enabled: true
  bind: 0.0.0.0
  port: 9876
  token: ""

zabbix:
  mode: api
  sender:
    server: ""
    port: 10051
    host: ""
    timeout_seconds: 30
    chunk_size: 250
    tls: none
    server_name: ""
    ca_file: ""
    cert_file: ""
    key_file: ""
    psk_identity: ""
    psk: ""

products:
  hyper_backup:
    enabled: false
    insecure_skip_verify: false
    api_url: ""
    username: ""
    password: ""
  active_backup_business:
    enabled: true
    scan_paths:
      - /volume*/@ActiveBackup
      - /volume*/ActiveBackupforBusiness
  active_backup_m365:
    enabled: true
    scan_paths:
      - /volume*/@ActiveBackup-Office365/db

logging:
  level: info

paths:
  package_root: /var/packages/synology-activebackup-zabbix
  cache_file: /var/packages/synology-activebackup-zabbix/var/cache/status.json
  sender_log_file: /var/packages/synology-activebackup-zabbix/var/cache/sender-log.json
  log_dir: /var/packages/synology-activebackup-zabbix/var/log
  run_dir: /var/packages/synology-activebackup-zabbix/var/run

privacy:
  redact_names: true
```

`api.token` is required for all `/api/v1/*` endpoints. During manual DSM installation the package asks for the token; if the field is not provided, the post-install script generates one automatically. The DSM desktop app shows the active API URL and token through DSM-authenticated CGI calls.

`zabbix.mode` can be `api` or `sender`. In sender mode, set `api.bind` to `127.0.0.1` so the DSM desktop app can still use the local API proxy while Zabbix receives active pushes through trapper items.

`zabbix.sender.host` must match the technical host name configured in Zabbix. `zabbix.sender.server` can point to a Zabbix server or proxy. `tls` supports `none`, `cert`, and `psk`; all sender variants use Go networking directly and do not require a native `zabbix_sender` binary on DSM.

`privacy.redact_names` avoids exposing Microsoft 365 user names and email addresses in discovered job names. Set it to `false` only when the Zabbix host is allowed to store those names.

Runtime changes still require a package restart because the listener, collector, and sender settings are read at service start. Stop and run the package again from DSM Package Center.

Hyper Backup is disabled by default, including when upgrading an existing configuration. Enable `products.hyper_backup.enabled` in YAML or select **Hyper Backup** under **Config → Products**, save, and restart the package. Set `api_url` to the DSM HTTPS origin and supply `username`/`password` for access as the standard package user. An empty `api_url` selects local `synowebapi` access, which may require privileges unavailable to that account. Hyper Backup has no database scan paths. See [Hyper Backup](HYPERBACKUP.md) for permissions and diagnostics.
