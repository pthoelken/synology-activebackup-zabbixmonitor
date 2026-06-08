module github.com/pthoelken/synology-activebackup-zabbixmonitor

go 1.22

replace github.com/jc-lab/go-tls-psk => ./third_party/go-tls-psk

require (
	github.com/jc-lab/go-tls-psk v1.18.3-r002
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.34.5
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/crypto v0.0.0-20220622213112-05595931fe9d // indirect
	golang.org/x/sys v0.22.0 // indirect
	modernc.org/libc v1.55.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
)
