# Zabbix

Import:

```text
zabbix/template_synology_activebackup_zabbix_7.4.yaml
```

Template name:

```text
Template Synology Active Backup Zabbix
```

The template uses two low-level discovery rules:

- Active Backup for Business Discovery
- Active Backup for Microsoft 365 Discovery

LLD macros:

- `{#PRODUCT}`
- `{#TASKID}`
- `{#JOBNAME}`
- `{#SERVICETYPE}`
- `{#BACKUPTYPE}`

The package writes `/etc/zabbix/zabbix_agent2.d/synology_activebackup.conf` during install. Restart Zabbix agent 2 after installing or upgrading the package if DSM does not reload it automatically.

Main threshold:

```text
{$ACTIVEBACKUP.MAX_AGE_HOURS}=30
```

Status value map:

- `1` = OK
- `6` = Failed
- `8` = No data
- `9` = DB missing
- `10` = Unknown
