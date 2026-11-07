#!/usr/bin/env bash
# XinuGate One-Step Installer for Linux
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/x-inu/XinuGate/main/install.sh | sudo bash
#   curl -fsSL https://raw.githubusercontent.com/x-inu/XinuGate/main/install.sh | bash -s -- --port 20128

set -euo pipefail

REPO="x-inu/XinuGate"
PORT="${XINUGATE_PORT:-20128}"
INSTALL_DIR="${XINUGATE_DIR:-}"
NO_SERVICE=0
UNINSTALL=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    --port)
      PORT="$2"
      shift 2
      ;;
    --no-service)
      NO_SERVICE=1
      shift
      ;;
    --uninstall)
      UNINSTALL=1
      shift
      ;;
    -h|--help)
      echo "Usage: install.sh [--dir /opt/xinugate] [--port 20128] [--no-service] [--uninstall]"
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

IS_ROOT=0
if [[ "$(id -u)" -eq 0 ]]; then
  IS_ROOT=1
fi

if [[ -z "${INSTALL_DIR}" ]]; then
  if [[ "${IS_ROOT}" -eq 1 ]]; then
    INSTALL_DIR="/opt/xinugate"
  else
    INSTALL_DIR="${HOME}/xinugate"
  fi
fi

if [[ "${UNINSTALL}" -eq 1 ]]; then
  echo "==> Uninstalling XinuGate..."
  if [[ "${IS_ROOT}" -eq 1 ]] && command -v systemctl >/dev/null 2>&1; then
    systemctl stop xinugate.service 2>/dev/null || true
    systemctl disable xinugate.service 2>/dev/null || true
    rm -f /etc/systemd/system/xinugate.service
    systemctl daemon-reload || true
  fi
  echo "==> Stopped service. Data in ${INSTALL_DIR} was kept safe (remove manually with: rm -rf ${INSTALL_DIR})."
  exit 0
fi

# 1. Detect OS & Architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [[ "${OS}" != "linux" ]]; then
  echo "Error: This automated installer supports Linux. For other platforms, build from source." >&2
  exit 1
fi

ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64|amd64)
    ASSET_ARCH="amd64"
    ;;
  aarch64|arm64)
    ASSET_ARCH="aarch64"
    ;;
  *)
    echo "Error: Unsupported CPU architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

# Use static no-plugin build on arm64 or musl/older glibc systems
ASSET_SUFFIX=""
if [[ "${ASSET_ARCH}" == "aarch64" ]]; then
  ASSET_SUFFIX="_no-plugin"
elif ! ldd --version 2>&1 | grep -qi "glibc\|gnu"; then
  ASSET_SUFFIX="_no-plugin"
fi

# 2. Resolve latest release tag
echo "==> Checking latest XinuGate release..."
TAG="$(
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n 1 || true
)"
if [[ -z "${TAG}" ]]; then
  TAG="v0.1.0"
fi
VER="${TAG#v}"
VER="${VER#xinu-v}"

ARCHIVE="XinuGate_${VER}_linux_${ASSET_ARCH}${ASSET_SUFFIX}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "==> Downloading ${ARCHIVE} (${TAG})..."
curl -fL "${BASE_URL}/${ARCHIVE}" -o "${TMP_DIR}/${ARCHIVE}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP_DIR}/checksums.txt"

echo "==> Verifying SHA-256 checksum..."
(
  cd "${TMP_DIR}"
  grep "  ${ARCHIVE}\$" checksums.txt | sha256sum -c -
)

mkdir -p "${INSTALL_DIR}"
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "${TMP_DIR}"

# 3. Install binary and example config
if [[ "${IS_ROOT}" -eq 1 ]] && command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet xinugate.service; then
  systemctl stop xinugate.service
fi

install -m 0755 "${TMP_DIR}/xinugate" "${INSTALL_DIR}/xinugate"
cp -f "${TMP_DIR}/config.example.yaml" "${INSTALL_DIR}/config.example.yaml"
cp -f "${TMP_DIR}/LICENSE" "${INSTALL_DIR}/LICENSE"
cp -f "${TMP_DIR}/README.md" "${INSTALL_DIR}/README.md"

# 4. Generate config.yaml on first install (never overwrite existing config.yaml)
random_hex() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex "$1"
  else
    od -An -N "$1" -tx1 /dev/urandom | tr -d ' \n'
  fi
}

NEW_CONFIG=0
MGMT_KEY=""
PROXY_KEY=""

if [[ ! -f "${INSTALL_DIR}/config.yaml" ]]; then
  NEW_CONFIG=1
  MGMT_KEY="xg-admin-$(random_hex 12)"
  PROXY_KEY="xg-key-$(random_hex 16)"

  cat > "${INSTALL_DIR}/config.yaml" <<EOF
# XinuGate Configuration
host: "0.0.0.0"
port: ${PORT}

remote-management:
  allow-remote: true
  secret-key: "${MGMT_KEY}"
  disable-control-panel: false
  disable-auto-update-panel: true
  panel-github-repository: "https://github.com/${REPO}"

auth-dir: "${INSTALL_DIR}/auths"

api-keys:
  - "${PROXY_KEY}"

public-models-endpoint: false
debug: false
usage-statistics-enabled: true
request-retry: 3
routing:
  strategy: "round-robin"
EOF
  chmod 0600 "${INSTALL_DIR}/config.yaml"
fi

# 5. Setup systemd service when running as root
SERVICE_ACTIVE=0
if [[ "${IS_ROOT}" -eq 1 && "${NO_SERVICE}" -eq 0 ]] && command -v systemctl >/dev/null 2>&1; then
  echo "==> Configuring systemd service (xinugate.service)..."
  cat > /etc/systemd/system/xinugate.service <<EOF
[Unit]
Description=XinuGate - Unified AI Gateway & OAuth Proxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
Environment=HOME=/root
Environment=SQLITESTORE=true
Environment=SQLITESTORE_PATH=${INSTALL_DIR}/data.sqlite
ExecStart=${INSTALL_DIR}/xinugate --config ${INSTALL_DIR}/config.yaml -sqlitestore -sqlitestore-path ${INSTALL_DIR}/data.sqlite
Restart=on-failure
RestartSec=3
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable --now xinugate.service
  sleep 1
  if systemctl is-active --quiet xinugate.service; then
    SERVICE_ACTIVE=1
  fi
fi

echo ""
echo "=================================================================="
echo "  XinuGate ${TAG} installed successfully!"
echo "=================================================================="
echo "  Install directory : ${INSTALL_DIR}"
echo "  Web UI & Gateway  : http://127.0.0.1:${PORT}/"
echo "  OpenAI Base URL   : http://127.0.0.1:${PORT}/v1"
echo ""
if [[ "${NEW_CONFIG}" -eq 1 ]]; then
  echo "  [SAVE THESE KEYS]"
  echo "  Web UI Login Key  : ${MGMT_KEY}"
  echo "  Client API Key    : ${PROXY_KEY}"
  echo ""
else
  echo "  Existing config.yaml preserved at ${INSTALL_DIR}/config.yaml"
  echo ""
fi
if [[ "${SERVICE_ACTIVE}" -eq 1 ]]; then
  echo "  Service status    : xinugate.service is RUNNING"
  echo "  Manage service    : systemctl status|restart|stop xinugate"
  echo "  View live logs    : journalctl -u xinugate -f"
else
  echo "  Start manually    :"
  echo "    cd ${INSTALL_DIR} && ./xinugate --config config.yaml -sqlitestore"
fi
echo "=================================================================="
