VERSION ?= 0.1.0

.PHONY: build spk clean

build:
	go build -trimpath -ldflags "-X main.version=$(VERSION)" ./cmd/synology-activebackup-zabbix

spk:
	VERSION=$(VERSION) ./packaging/scripts/build-spk.sh

clean:
	rm -rf build dist synology-activebackup-zabbix
