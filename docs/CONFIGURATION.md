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

zabbix:
  mode: agent
  userparameter_file: /etc/zabbix/zabbix_agent2.d/synology_activebackup.conf

http:
  enabled: true
  bind: 127.0.0.1
  port: 9876

products:
  active_backup_business:
    enabled: true
    scan_paths:
      - /volume*/@ActiveBackup
      - /volume*/ActiveBackupforBusiness
      - /volume*/@ActiveBackup*/db
  active_backup_m365:
    enabled: true
    scan_paths:
      - /volume*/@ActiveBackup-Office365/db

logging:
  level: info

privacy:
  redact_names: true
```

`privacy.redact_names` avoids exposing Microsoft 365 user names and email addresses in discovered job names. Set it to `false` only when the Zabbix host is allowed to store those names.
