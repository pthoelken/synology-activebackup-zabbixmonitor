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

GitHub releases are created automatically for every push to `main` by `.github/workflows/release.yml`. The workflow runs tests, validates the DSM app JavaScript and Zabbix templates, builds both SPK architectures, uploads the build artifact, and publishes a GitHub Release with generated release notes and both SPK files attached.

The build script creates:

- `dist/synology-activebackup-zabbixmonitor-0.1.13-x86_64.spk`
- `dist/synology-activebackup-zabbixmonitor-0.1.13-aarch64.spk`

Set a release version with:

```sh
make spk VERSION=0.2.0
```

The SPK contains `INFO`, package icons, `conf/privilege`, `WIZARD_UIFILES`, Synology package scripts, `package.tgz`, the binary, DSM desktop app files, default config, docs, and the Zabbix templates.

DSM recognizes manual installs as package updates when the `package` value in `INFO` stays the same and the new `version` is higher than the installed version.
