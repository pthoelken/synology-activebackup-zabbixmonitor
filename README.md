# Synology Active Backup Zabbix Monitor

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Synology DSM](https://img.shields.io/badge/Synology%20DSM-7.4%2B-0086C3?style=for-the-badge&logo=synology&logoColor=white)](https://www.synology.com/dsm)
[![Zabbix](https://img.shields.io/badge/Zabbix-8.0beta2-D40000?style=for-the-badge&logo=zabbix&logoColor=white)](https://www.zabbix.com/)
[![SPK](https://img.shields.io/badge/SPK-x86__64%20%7C%20aarch64-4B5563?style=for-the-badge)](#build)
[![License: MIT](https://img.shields.io/badge/License-MIT-2EA44F?style=for-the-badge)](LICENSE)

DSM 7 package project for monitoring Synology Active Backup for Business and Active Backup for Microsoft 365 with Zabbix 7.4.

The collector is implemented in Go because the package needs a long-running daemon, a CLI, structured JSON cache output, and cross-architecture builds for DSM. A single Go binary keeps DSM packaging simpler than a multi-script solution and avoids requiring Python, Node.js, or shell-only parsing on the NAS.

<a id="table-of-contents"></a>

## Table of Contents 📚

- [Overview](#overview)
- [Highlights](#highlights)
- [Quick Start](#quick-start)
- [What It Monitors](#what-it-monitors)
- [Runtime Paths](#runtime-paths)
- [DSM Desktop App](#dsm-desktop-app)
- [Zabbix Integration](#zabbix-integration)
- [API](#api)
- [Test the API](#test-the-api)
- [CLI](#cli)
- [Build](#build)
- [Documentation](#documentation)
- [License](#license)

<a id="overview"></a>

## Overview ✨

| Area          | Details                                                     |
| ------------- | ----------------------------------------------------------- |
| Package       | DSM 7 SPK package                                           |
| Products      | Active Backup for Business, Active Backup for Microsoft 365 |
| Monitoring    | Zabbix 7.4 templates for API pull and sender push           |
| Runtime       | Single Go daemon with CLI helpers and JSON cache output     |
| Architectures | `x86_64`, `aarch64`                                         |

<a id="highlights"></a>

## Highlights ✅

- Ships as a Synology DSM package with a desktop app named `Active Backup Zabbix`.
- Supports `API pull` via token-protected HTTP endpoints.
- Supports `Sender push` to Zabbix trapper items, including plain TCP, certificate TLS, and TLS PSK.
- Keeps the package API local in sender mode by binding to `127.0.0.1`.
- Includes Zabbix 7.4 templates for both monitoring modes.
- Builds SPK artifacts for `x86_64` and `aarch64`.

<a id="quick-start"></a>

## Quick Start 🚀

1. Build or download the SPK for your NAS architecture and install it through DSM Package Center with `Manual Install`.
2. Start the package and open the DSM desktop app `Active Backup Zabbix`.
3. Choose a Zabbix mode in `Config`.
4. For `Sender push`, import `zabbix/template_synology_activebackup_zabbix_sender_7.4.yaml`, link it to the Zabbix host, set the exact technical Zabbix host name in the DSM app, and configure the Zabbix server or proxy address.
5. If sender mode uses TLS PSK, enable PSK encryption on the Zabbix host under `Encryption` and enter the same PSK identity and PSK value as in the DSM app.
6. For `API pull`, import `zabbix/template_synology_activebackup_zabbix_7.4.yaml`, link it to the Zabbix host, and set `{$ACTIVEBACKUP.API.URL}` plus `{$ACTIVEBACKUP.API.TOKEN}`.
7. Save the DSM app config, restart the package from Package Center when the UI asks for it, and check the DSM app `Log` tab plus Zabbix `Latest data`.

`Sender push` is recommended when the NAS should send values to Zabbix and the package API should stay local. `API pull` is useful when Zabbix should collect values from the NAS API directly.

<a id="what-it-monitors"></a>

## What It Monitors 🔎

- Active Backup for Microsoft 365 from `/volume*/@ActiveBackup-Office365/db/log.sqlite`
- Active Backup for Business by scanning likely ABB SQLite databases below `/volume*/@ActiveBackup` and `/volume*/ActiveBackupforBusiness`

For Microsoft 365, completed runs with skipped items are treated as Warning, not Failed.

<a id="runtime-paths"></a>

## Runtime Paths 🗂️

| Purpose               | Path                                                               |
| --------------------- | ------------------------------------------------------------------ |
| Package root          | `/var/packages/synology-activebackup-zabbix`                       |
| Config                | `/var/packages/synology-activebackup-zabbix/etc/config.yml`        |
| Logs                  | `/var/packages/synology-activebackup-zabbix/var/log`               |
| Cache                 | `/var/packages/synology-activebackup-zabbix/var/cache/status.json` |
| API                   | `http://<nas>:9876/api/v1`                                         |
| Local sender-mode API | `http://127.0.0.1:9876/api/v1`                                     |

<a id="dsm-desktop-app"></a>

## DSM Desktop App 🖥️

The SPK installs a DSM desktop application named `Active Backup Zabbix`. It opens as a DSM desktop window through the package `dsmuidir` integration. The UI uses DSM-authenticated CGI calls and does not expose static status JSON files.

<a id="zabbix-integration"></a>

## Zabbix Integration 📡

Two Zabbix modes are available:

| Mode     | Flow                                                                    | Best fit                                                                    |
| -------- | ----------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| `api`    | Zabbix HTTP agent items pull data from the token-protected package API. | Zabbix can reach the NAS API directly.                                      |
| `sender` | The NAS pushes data to Zabbix server or proxy trapper items.            | The package API should stay local or firewalls make pull mode inconvenient. |

For pull mode, import `zabbix/template_synology_activebackup_zabbix_7.4.yaml` in Zabbix 7.4 and link it to the Synology host. Set `{$ACTIVEBACKUP.API.URL}` to the package API base URL, for example `http://192.168.178.240:9876`, and `{$ACTIVEBACKUP.API.TOKEN}` to the token from the DSM desktop app.

For active sender mode, import `zabbix/template_synology_activebackup_zabbix_sender_7.4.yaml`, link it to the same host, and set `{$ACTIVEBACKUP.SENDER.ALLOWED_HOSTS}` to the NAS address that connects to Zabbix if `{HOST.CONN}` is not that address.

When sender mode uses TLS PSK, also enable encryption on the Zabbix host itself: open the host `Encryption` tab, allow `Connections from host` with `PSK`, and enter the same PSK identity and PSK value as in the DSM app. If this is missing, the sender connection can succeed but Zabbix rejects the submitted values.

Both templates include a no-data trigger for `synology.activebackup.health`. By default it raises an alarm when no values are received for `30m`, which indicates that the DSM package, API, or sender flow is no longer delivering data.

<a id="api"></a>

## API 🔐

The package service starts a token-protected HTTP API for Zabbix:

| Endpoint                                             | Purpose                                                 |
| ---------------------------------------------------- | ------------------------------------------------------- |
| `GET /api/v1/ping`                                   | Authenticated reachability check                        |
| `GET /api/v1/status`                                 | Full status payload                                     |
| `GET /api/v1/discovery`                              | Low-level discovery for all products                    |
| `GET /api/v1/discovery?product=abb`                  | Low-level discovery for Active Backup for Business      |
| `GET /api/v1/discovery?product=m365`                 | Low-level discovery for Active Backup for Microsoft 365 |
| `GET /api/v1/health?field=ok`                        | Single health field                                     |
| `GET /api/v1/summary?field=total`                    | Single summary field                                    |
| `GET /api/v1/job?product=abb&task_id=1&field=status` | Single job field                                        |

All API endpoints require `Authorization: Bearer <token>`. The token is configured during package installation and is visible in the DSM desktop app config tab.

<a id="test-the-api"></a>

## Test the API 🧪

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

For Postman, create a `GET` request to `http://NAS_IP:9876/api/v1/status`, choose `Bearer Token` in `Authorization`, paste the DSM app token, and send the request. A successful response is JSON with `health`, `jobs`, and `sources`.

<a id="cli"></a>

## CLI 🛠️

```sh
synology-activebackup-zabbix discovery
synology-activebackup-zabbix status
synology-activebackup-zabbix job --product m365 --task-id 1
synology-activebackup-zabbix health
synology-activebackup-zabbix detect
synology-activebackup-zabbix send --fresh
```

Use `--config /path/to/config.yml` before the command to override the config file.

<a id="build"></a>

## Build 📦

Install Go 1.22 or newer, then run:

```sh
make spk
```

The SPK files are written to `dist/` for `x86_64` and `aarch64`.

<a id="documentation"></a>

## Documentation 📖

- [Build notes](docs/BUILD.md)
- [Configuration](docs/CONFIGURATION.md)
- [Security](docs/SECURITY.md)
- [Zabbix details](docs/ZABBIX.md)

<a id="license"></a>

## License 🔓

This project is released under the [MIT License](LICENSE). Vendored third-party code in `third_party/go-tls-psk` keeps its own BSD 3-Clause license.
