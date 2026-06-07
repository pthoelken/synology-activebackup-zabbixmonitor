# Security

The collector opens Synology SQLite databases read-only. It first uses normal read-only mode so SQLite can consider WAL files, then falls back to immutable read-only mode when a database is locked.

The collector never writes to Active Backup databases.

The DSM package scripts run as root during install because they must create package directories and write the Zabbix agent UserParameter file below `/etc/zabbix`. The daemon itself can run without root if the configured user has read access to the Active Backup SQLite files and write access to the package cache and log paths. On many DSM systems, reading Active Backup internals requires root or an elevated package context.

Logging is structured JSON. Debug logs must not contain tenant IDs, usernames, or email addresses. Microsoft 365 job names are redacted by default through `privacy.redact_names: true`.
