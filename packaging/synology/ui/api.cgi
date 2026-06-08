#!/bin/sh

exec /var/packages/synology-activebackup-zabbix/target/bin/synology-activebackup-zabbix \
  --config /var/packages/synology-activebackup-zabbix/etc/config.yml \
  dsm-cgi
