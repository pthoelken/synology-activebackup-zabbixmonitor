# synology-activebackup-zabbixmonitor

DSM 7 package project for monitoring Synology Active Backup for Business and Active Backup for Microsoft 365 with Zabbix 7.4.

The collector is implemented in Go because the package needs a long-running daemon, a CLI, structured JSON cache output, and cross-architecture builds for DSM. A single Go binary keeps DSM packaging simpler than a multi-script solution and avoids requiring Python, Node.js, or shell-only parsing on the NAS.

## What It Monitors

- Active Backup for Microsoft 365 from `/volume*/@ActiveBackup-Office365/db/log.sqlite`
- Active Backup for Business by scanning likely ABB SQLite databases below:
  - `/volume*/@ActiveBackup`
  - `/volume*/ActiveBackupforBusiness`

For Microsoft 365, completed runs with skipped items are treated as Warning, not Failed.

## Runtime Paths

- Package root: `/var/packages/synology-activebackup-zabbix`
- Config: `/var/packages/synology-activebackup-zabbix/etc/config.yml`
- Logs: `/var/packages/synology-activebackup-zabbix/var/log`
- Cache: `/var/packages/synology-activebackup-zabbix/var/cache/status.json`
- API: `http://<nas>:9876/api/v1` or `http://127.0.0.1:9876/api/v1` in active sender mode

## DSM Desktop App

The SPK installs a DSM desktop application named `Active Backup Zabbix`. It opens as a DSM desktop window through the package `dsmuidir` integration. The UI uses DSM-authenticated CGI calls and does not expose static status JSON files.

## Zabbix Integration

Two Zabbix modes are available:

- `api`: Zabbix HTTP agent items pull data from the token-protected package API.
- `sender`: the NAS pushes data to Zabbix server or proxy trapper items. This avoids exposing the API port to the network; keep the API bind address at `127.0.0.1` for the DSM desktop app.

The sender mode uses Go networking directly and supports plain TCP, certificate TLS, and TLS PSK without requiring a `zabbix_sender` binary on DSM.

## API

The package service starts a token-protected HTTP API for Zabbix:

- `GET /api/v1/status`
- `GET /api/v1/discovery`
- `GET /api/v1/discovery?product=abb`
- `GET /api/v1/discovery?product=m365`
- `GET /api/v1/health?field=ok`
- `GET /api/v1/summary?field=total`
- `GET /api/v1/job?product=abb&task_id=1&field=status`

All API endpoints require `Authorization: Bearer <token>`. The token is configured during package installation and is visible in the DSM desktop app config tab.

## Test the API

Replace `NAS_IP` and `TOKEN` with the values from the DSM desktop app config tab.

```sh
curl -i http://NAS_IP:9876/api/v1/status
```

Without a token this must return `401`.

```sh
curl -sS \
  -H "Authorization: Bearer TOKEN" \
  http://NAS_IP:9876/api/v1/ping

curl -sS \
  -H "Authorization: Bearer TOKEN" \
  http://NAS_IP:9876/api/v1/status

curl -sS \
  -H "Authorization: Bearer TOKEN" \
  "http://NAS_IP:9876/api/v1/discovery"

curl -sS \
  -H "Authorization: Bearer TOKEN" \
  "http://NAS_IP:9876/api/v1/discovery?product=m365"

curl -sS \
  -H "Authorization: Bearer TOKEN" \
  "http://NAS_IP:9876/api/v1/summary?field=total"
```

For Postman:

1. Create a `GET` request to `http://NAS_IP:9876/api/v1/status`.
2. Open `Authorization`, choose `Bearer Token`, and paste the DSM app token.
3. Send the request. A successful response is JSON with `health`, `jobs`, and `sources`.

## CLI

```sh
synology-activebackup-zabbix discovery
synology-activebackup-zabbix status
synology-activebackup-zabbix job --product m365 --task-id 1
synology-activebackup-zabbix health
synology-activebackup-zabbix detect
synology-activebackup-zabbix send --fresh
```

Use `--config /path/to/config.yml` before the command to override the config file.

## Zabbix

For pull mode, import `zabbix/template_synology_activebackup_zabbix_7.4.yaml` in Zabbix 7.4 and link it to the Synology host. Set these host macros:

- `{$ACTIVEBACKUP.API.URL}` to the package API base URL, for example `http://192.168.178.240:9876`
- `{$ACTIVEBACKUP.API.TOKEN}` to the token from the DSM desktop app

For active sender mode, import `zabbix/template_synology_activebackup_zabbix_sender_7.4.yaml`, link it to the same host, and set `{$ACTIVEBACKUP.SENDER.ALLOWED_HOSTS}` to the NAS address that connects to Zabbix if `{HOST.CONN}` is not that address.

## Build

Install Go 1.22 or newer, then run:

```sh
make spk
```

The SPK files are written to `dist/` for `x86_64` and `aarch64`.
