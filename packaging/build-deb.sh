#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." >/dev/null 2>&1 && pwd)"

PKG="binance-pivot-monitor"

VERSION="${1:-}"
if [[ -z "${VERSION}" ]]; then
  if command -v git >/dev/null 2>&1 && [[ -d "${REPO_ROOT}/.git" ]]; then
    VERSION="$(git -C "${REPO_ROOT}" describe --tags --always --dirty 2>/dev/null || true)"
  fi
  if [[ -z "${VERSION}" ]]; then
    VERSION="$(date -u +%Y%m%d%H%M%S)"
  fi
fi
VERSION="${VERSION#v}"
VERSION="$(echo "${VERSION}" | sed -E 's/[^0-9A-Za-z.+~:-]+/-/g')"

if ! command -v dpkg-deb >/dev/null 2>&1; then
  echo "dpkg-deb not found. Run this on Debian/Ubuntu (apt-get install dpkg-dev)." >&2
  exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  echo "go not found. Install Go 1.22+." >&2
  exit 1
fi

ARCH="${ARCH:-$(dpkg --print-architecture)}"
GOARCH=""
GOARM=""
case "${ARCH}" in
  amd64)
    GOARCH=amd64
    ;;
  arm64)
    GOARCH=arm64
    ;;
  armhf)
    GOARCH=arm
    GOARM=7
    ;;
  i386)
    GOARCH=386
    ;;
  *)
    echo "Unsupported ARCH: ${ARCH}" >&2
    exit 1
    ;;
esac

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

STAGE="${WORKDIR}/stage"
mkdir -p "${STAGE}/DEBIAN"

cat >"${STAGE}/DEBIAN/control" <<EOF
Package: ${PKG}
Version: ${VERSION}
Section: net
Priority: optional
Architecture: ${ARCH}
Maintainer: Pivot Monitor <root@localhost>
Depends: ca-certificates, adduser, systemd
Description: Binance pivot monitor backend (signals + SSE + dashboard)
EOF

cat >"${STAGE}/DEBIAN/conffiles" <<EOF
/etc/binance-pivot-monitor/binance-pivot-monitor.env
EOF

install -d "${STAGE}/usr/bin" \
  "${STAGE}/etc/binance-pivot-monitor" \
  "${STAGE}/lib/systemd/system" \
  "${STAGE}/var/lib/binance-pivot-monitor"

echo "Building Go binary (linux/${GOARCH})..." >&2
pushd "${REPO_ROOT}" >/dev/null
CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH}" GOARM="${GOARM}" \
  go build -trimpath -ldflags "-s -w" \
  -o "${STAGE}/usr/bin/binance-pivot-monitor" ./cmd/server
popd >/dev/null

install -m 0755 "${SCRIPT_DIR}/binance-pivot-monitor-run.sh" "${STAGE}/usr/bin/binance-pivot-monitor-run"
install -m 0644 "${SCRIPT_DIR}/binance-pivot-monitor.service" "${STAGE}/lib/systemd/system/binance-pivot-monitor.service"
install -m 0644 "${SCRIPT_DIR}/binance-pivot-monitor.env" "${STAGE}/etc/binance-pivot-monitor/binance-pivot-monitor.env"
install -m 0755 "${SCRIPT_DIR}/debian-postinst.sh" "${STAGE}/DEBIAN/postinst"
install -m 0755 "${SCRIPT_DIR}/debian-prerm.sh" "${STAGE}/DEBIAN/prerm"

OUTDIR="${REPO_ROOT}/dist"
mkdir -p "${OUTDIR}"
OUTFILE="${OUTDIR}/${PKG}_${VERSION}_${ARCH}.deb"

dpkg-deb --build --root-owner-group "${STAGE}" "${OUTFILE}" >/dev/null

echo "Built: ${OUTFILE}" >&2
