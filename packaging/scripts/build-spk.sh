#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
VERSION="${VERSION:-0.1.13}"
DIST_DIR="${ROOT_DIR}/dist"
BUILD_DIR="${ROOT_DIR}/build/spk"
PACKAGE_NAME="synology-activebackup-zabbix"
SPK_BASENAME="synology-activebackup-zabbixmonitor"

mkdir -p "${DIST_DIR}"
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

build_one() {
  GOARCH_VALUE="$1"
  SYNO_ARCH="$2"
  WORK_DIR="${BUILD_DIR}/${SYNO_ARCH}"
  PACKAGE_DIR="${WORK_DIR}/package"
  SPK_DIR="${WORK_DIR}/spk"
  OUT_FILE="${DIST_DIR}/${SPK_BASENAME}-${VERSION}-${SYNO_ARCH}.spk"

  mkdir -p "${PACKAGE_DIR}/bin" "${PACKAGE_DIR}/etc" "${PACKAGE_DIR}/docs" "${PACKAGE_DIR}/zabbix" "${SPK_DIR}"

  CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH_VALUE}" go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o "${PACKAGE_DIR}/bin/synology-activebackup-zabbix" \
    "${ROOT_DIR}/cmd/synology-activebackup-zabbix"

  cp "${ROOT_DIR}/packaging/synology/templates/config.yml" "${PACKAGE_DIR}/etc/config.yml"
  cp "${ROOT_DIR}/zabbix/"*.yaml "${PACKAGE_DIR}/zabbix/"
  cp "${ROOT_DIR}/README.md" "${PACKAGE_DIR}/docs/README.md"
  cp "${ROOT_DIR}/docs/"*.md "${PACKAGE_DIR}/docs/"
  cp -R "${ROOT_DIR}/packaging/synology/ui" "${PACKAGE_DIR}/ui"
  rm -rf "${PACKAGE_DIR}/ui/web/data"
  chmod 755 "${PACKAGE_DIR}/ui/api.cgi"

  tar -C "${PACKAGE_DIR}" -czf "${SPK_DIR}/package.tgz" .
  sed -e "s/^version=.*/version=\"${VERSION}\"/" -e "s/^arch=.*/arch=\"${SYNO_ARCH}\"/" \
    "${ROOT_DIR}/packaging/synology/INFO" >"${SPK_DIR}/INFO"
  cp -R "${ROOT_DIR}/packaging/synology/scripts" "${SPK_DIR}/scripts"
  cp -R "${ROOT_DIR}/packaging/synology/conf" "${SPK_DIR}/conf"
  cp -R "${ROOT_DIR}/packaging/synology/WIZARD_UIFILES" "${SPK_DIR}/WIZARD_UIFILES"
  cp "${ROOT_DIR}/packaging/synology/PACKAGE_ICON.PNG" "${SPK_DIR}/PACKAGE_ICON.PNG"
  cp "${ROOT_DIR}/packaging/synology/PACKAGE_ICON_256.PNG" "${SPK_DIR}/PACKAGE_ICON_256.PNG"
  chmod 755 "${SPK_DIR}/scripts/"*
  tar -C "${SPK_DIR}" -cf "${OUT_FILE}" INFO package.tgz scripts conf WIZARD_UIFILES PACKAGE_ICON.PNG PACKAGE_ICON_256.PNG
  echo "${OUT_FILE}"
}

build_one amd64 x86_64
build_one arm64 aarch64
