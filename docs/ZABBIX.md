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

When replacing an older imported version, either delete the old template first or enable deletion of missing template elements during import. Version `0.1.13` keeps the API template and adds the sender/trapper template.

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

After saving sender settings, restart the DSM package. Normal operation sends automatically after each collection interval; the DSM app `Log` tab shows the last sender attempts and the values that were sent.

Main threshold:

```text
{$ACTIVEBACKUP.MAX_AGE_HOURS}=30
```

Status value map:

- `1` = OK
- `2` = Warning
- `3` = Running
- `6` = Failed
- `8` = No data
- `9` = DB missing
- `10` = Unknown
