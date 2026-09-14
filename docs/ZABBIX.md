# Zabbix

Import:

Pull/API mode:

```text
zabbix/template_synology_activebackup_zabbix_7.4.yaml
```

Active sender mode:

```text
zabbix/template_synology_activebackup_zabbix_sender_7.4.yaml
```

When replacing an older imported version, either delete the old template first or enable deletion of missing template elements during import. Version `0.1.16` keeps the API template and adds the sender/trapper template.

Template name:

```text
Template Synology Active Backup Zabbix
```

Both templates use one low-level discovery rule for all supported products:

- Active Backup Discovery

LLD macros:

- `{#PRODUCT}`
- `{#TASKID}`
- `{#JOBNAME}`
- `{#SERVICETYPE}`
- `{#BACKUPTYPE}`

The API template uses Zabbix HTTP agent items. Zabbix server or the assigned proxy connects to the package API directly; no Zabbix agent UserParameter and no SSH command on the NAS are required.

The sender template uses Zabbix trapper items. The DSM package sends discovery and item values to the Zabbix server or proxy on port `10051`, so the package API can stay bound to `127.0.0.1`.

Host macros:

API template:

- `{$ACTIVEBACKUP.API.URL}`: package API base URL, for example `http://192.168.178.240:9876`
- `{$ACTIVEBACKUP.API.TOKEN}`: bearer token from the DSM desktop app

Sender template:

- `{$ACTIVEBACKUP.SENDER.ALLOWED_HOSTS}`: NAS source IP or DNS name allowed by trapper items; default is `{HOST.CONN}`

In the API template, the token macro is exported as a normal text macro for broad import compatibility. After importing the template you can change the host macro type to `Secret text` in Zabbix.

The token is sent as:

```text
Authorization: Bearer {$ACTIVEBACKUP.API.TOKEN}
```

For sender mode, configure the DSM app:

- `Zabbix mode`: `sender`
- `Zabbix server or proxy`: address of your Zabbix server or proxy
- `Zabbix trapper port`: usually `10051`
- `Zabbix host name`: the technical Zabbix host name
- `Zabbix TLS`: `none`, `cert`, or `psk`
- `PSK identity` and `PSK value`: only for `psk`

For TLS PSK sender mode, configure encryption on the Zabbix host too: open the host `Encryption` tab, allow `Connections from host` with `PSK`, and enter the same PSK identity and PSK value as in the DSM app. Without this, Zabbix can accept the connection but reject all submitted trapper values.

After saving sender settings, restart the DSM package. Normal operation sends automatically after each collection interval; the DSM app `Log` tab shows the last sender attempts and the values that were sent.

Sender troubleshooting:

- `response: success` with `processed: 0; failed: N; total: N` means the TCP/TLS/PSK connection worked, but Zabbix rejected the values.
- Check that `Zabbix host name` in the DSM app is the technical Zabbix host name, not only the visible display name.
- Check that the sender/trapper template is linked to that exact host.
- Check that `{$ACTIVEBACKUP.SENDER.ALLOWED_HOSTS}` matches the source address Zabbix sees for the NAS.
- With logging level `debug`, the DSM app `Log` tab shows diagnostics, chunk responses, and a larger sent-value preview. The package uses a Go sender implementation, so there is no external `zabbix_sender` binary output on DSM.

Main threshold:

```text
{$ACTIVEBACKUP.MAX_AGE_HOURS}=30
```

No-data threshold:

```text
{$ACTIVEBACKUP.NO_DATA_PERIOD}=30m
```

Status value map:

- `1` = OK
- `2` = Warning
- `3` = Running
- `6` = Failed
- `8` = No data
- `9` = DB missing
- `10` = Unknown

These are normalized monitoring statuses, not the raw status codes stored in the Synology databases. For Active Backup for Business, warning and partial-completion results, including raw status codes `5` and `8`, are normalized to status `2`. A raw ABB status of `2` remains a successful result and is normalized to status `1`.

ABB device results are matched to the corresponding device before their status is normalized. This prevents the status of another device in the same backup task from being reported for the discovered job. Both supplied Zabbix templates already raise their backup warning trigger when the normalized job status is `2`, so this behavior does not require a template change.
