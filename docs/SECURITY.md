# Security

The collector opens Synology SQLite databases read-only. It first uses normal read-only mode so SQLite can consider WAL files, then falls back to immutable read-only mode when a database is locked.

The collector never writes to Active Backup databases.

The DSM package declares `conf/privilege` with `run-as: package`, so Package Center installs and runs it as the package user instead of root. The daemon can collect data when that package user has read access to the Active Backup SQLite files and write access to the package cache and log paths. On some DSM systems, reading Active Backup internals may still require additional permissions for the package user.

The package does not write below `/etc/zabbix` during install. Pull mode uses HTTP agent checks against the package API, authenticated with a bearer token. Sender mode pushes values to Zabbix trapper items and can keep the package API bound to `127.0.0.1`, so no package API port has to be reachable from the network.

The package opens only the configured JSON API port. All `/api/v1/*` endpoints require `Authorization: Bearer <token>` or `X-API-Token: <token>` and return `401` without it. The DSM desktop app does not call that port from the browser; it calls `api.cgi`, which first validates the DSM session and then proxies to the local API server-side. The HTML status UI is loaded only inside an isolated iframe below `ui/web`, so its CSS and scripts do not run in the DSM desktop document. Direct top-level browser launches of the UI page are blocked client-side; users should open it through DSM.

TLS PSK sender mode is implemented in Go and does not require a native `zabbix_sender` binary on DSM. The PSK value is stored in the package config and is masked in the DSM app unless explicitly revealed.

Logging is structured JSON. Debug logs must not contain tenant IDs, usernames, or email addresses. Microsoft 365 job names are redacted by default through `privacy.redact_names: true`.
