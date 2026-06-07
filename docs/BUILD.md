# Build

Requirements:

- Go 1.22 or newer
- `tar`
- Network access for the first `go mod download`

Build local binary:

```sh
make build
```

Build Synology packages:

```sh
make spk
```

The build script creates:

- `dist/synology-activebackup-zabbixmonitor-0.1.0-x86_64.spk`
- `dist/synology-activebackup-zabbixmonitor-0.1.0-aarch64.spk`

Set a release version with:

```sh
make spk VERSION=0.2.0
```

The SPK contains `INFO`, Synology package scripts, `package.tgz`, the binary, default config, docs, and the Zabbix template.
