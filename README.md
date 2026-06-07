# synology-activebackup-zabbixmonitor

DSM 7 package project for monitoring Synology Active Backup for Business and Active Backup for Microsoft 365 with Zabbix 7.4.

The collector is implemented in Go because the package needs a long-running daemon, a CLI, an optional local HTTP API, structured JSON cache output, and cross-architecture builds for DSM. A single Go binary keeps DSM packaging simpler than a multi-script solution and avoids requiring Python, Node.js, or shell-only parsing on the NAS.

## What It Monitors

- Active Backup for Microsoft 365 from `/volume*/@ActiveBackup-Office365/db/log.sqlite`
- Active Backup for Business by scanning likely ABB SQLite databases below:
  - `/volume*/@ActiveBackup`
  - `/volume*/ActiveBackupforBusiness`
  - `/volume*/@ActiveBackup*/db`

For Microsoft 365, `execution_status = 1` is treated as OK and `execution_status = 6` as failed. Every other status is a problem and maps to `Unknown`.

## Runtime Paths

- Package root: `/var/packages/synology-activebackup-zabbix`
- Config: `/var/packages/synology-activebackup-zabbix/etc/config.yml`
- Logs: `/var/packages/synology-activebackup-zabbix/var/log`
- Cache: `/var/packages/synology-activebackup-zabbix/var/cache/status.json`
- UserParameter file: `/etc/zabbix/zabbix_agent2.d/synology_activebackup.conf`

## CLI

```sh
synology-activebackup-zabbix discovery
synology-activebackup-zabbix status
synology-activebackup-zabbix job --product m365 --task-id 1
synology-activebackup-zabbix health
synology-activebackup-zabbix detect
```

Use `--config /path/to/config.yml` before the command to override the config file.

## HTTP API

Default bind: `127.0.0.1:9876`

- `GET /health`
- `GET /zabbix/discovery`
- `GET /zabbix/job/{product}/{task_id}`
- `GET /api/status`
- `GET /api/jobs`

## Zabbix

Import `zabbix/template_synology_activebackup_zabbix_7.4.yaml` in Zabbix 7.4 and link it to the Synology host that runs Zabbix agent 2.

The package writes these keys through Zabbix UserParameter:

- `synology.activebackup.discovery`
- `synology.activebackup.discovery[*]`
- `synology.activebackup.job.status[*]`
- `synology.activebackup.job.age[*]`
- `synology.activebackup.job.last_success_age[*]`
- `synology.activebackup.job.error[*]`
- `synology.activebackup.job.runtime[*]`
- `synology.activebackup.job.transferred_size[*]`
- `synology.activebackup.job.last_end_time[*]`
- `synology.activebackup.job.info[*]`
- `synology.activebackup.health`
- `synology.activebackup.product.db_missing[*]`
- `synology.activebackup.jobs.successful`
- `synology.activebackup.jobs.failed`
- `synology.activebackup.jobs.problem`
- `synology.activebackup.jobs.total`

## Build

Install Go 1.22 or newer, then run:

```sh
make spk
```

The SPK files are written to `dist/` for `x86_64` and `aarch64`.
