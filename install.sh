#!/usr/bin/env sh
# XinuGate — One-Command POSIX sh Installer for Linux
# Usage:
#   curl -fsSL https://raw.xinu.my.id/xgate | sh
#   curl -fsSL https://raw.xinu.my.id/xgate | sh -s -- --port 20128

set -eu

REPO="x-inu/XinuGate"
PORT="${XINUGATE_PORT:-20128}"
INSTALL_DIR="${XINUGATE_DIR:-}"
NO_SERVICE=0
UNINSTALL=0

while [ "$#" -gt 0 ]; do
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
      echo "Usage: xgate [--dir /opt/xinugate] [--port 20128] [--no-service] [--uninstall]"
      exit 0
      ;;
    *)
      echo "Error: Unknown option '$1'" >&2
      exit 1
      ;;
  esac
done

IS_ROOT=0
SUDO=""
if [ "$(id -u)" -eq 0 ]; then
  IS_ROOT=1
elif command -v sudo >/dev/null 2>&1; then
  if sudo -n true 2>/dev/null; then
    SUDO="sudo"
    IS_ROOT=1
  elif [ -r /dev/tty ]; then
    echo "==> Requesting sudo privileges to install XinuGate service..."
    if sudo true </dev/tty; then
      SUDO="sudo"
      IS_ROOT=1
    fi
  fi
fi

if [ -z "${INSTALL_DIR}" ]; then
  if [ "${IS_ROOT}" -eq 1 ]; then
    INSTALL_DIR="/opt/xinugate"
  else
    INSTALL_DIR="${HOME}/xinugate"
  fi
fi

run_priv() {
  if [ -n "${SUDO}" ]; then
    "${SUDO}" "$@"
  else
    "$@"
  fi
}

if [ "${UNINSTALL}" -eq 1 ]; then
  echo "==> Uninstalling XinuGate service..."
  if [ "${IS_ROOT}" -eq 1 ] && command -v systemctl >/dev/null 2>&1; then
    run_priv systemctl stop xinugate.service 2>/dev/null || true
    run_priv systemctl disable xinugate.service 2>/dev/null || true
    run_priv rm -f /etc/systemd/system/xinugate.service
    run_priv systemctl daemon-reload || true
  fi
  echo "==> Stopped service. Data in ${INSTALL_DIR} was kept safe (remove with: rm -rf ${INSTALL_DIR})."
  exit 0
fi

# 1. Detect OS & CPU Architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [ "${OS}" != "linux" ]; then
  echo "Error: This installer supports Linux. For other platforms, build from source." >&2
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

ASSET_SUFFIX=""
if [ "${ASSET_ARCH}" = "aarch64" ]; then
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
if [ -z "${TAG}" ]; then
  TAG="v0.1.0"
fi
VER="${TAG#v}"
VER="${VER#xinu-v}"

ARCHIVE="XinuGate_${VER}_linux_${ASSET_ARCH}${ASSET_SUFFIX}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT INT TERM

echo "==> Downloading ${ARCHIVE} (${TAG})..."
curl -fL "${BASE_URL}/${ARCHIVE}" -o "${TMP_DIR}/${ARCHIVE}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP_DIR}/checksums.txt"

echo "==> Verifying SHA-256 checksum..."
(
  cd "${TMP_DIR}"
  grep "  ${ARCHIVE}\$" checksums.txt | sha256sum -c -
)

run_priv mkdir -p "${INSTALL_DIR}"
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "${TMP_DIR}"

# 3. Install binary and reference files
if [ "${IS_ROOT}" -eq 1 ] && command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet xinugate.service; then
  run_priv systemctl stop xinugate.service
fi

run_priv install -m 0755 "${TMP_DIR}/xinugate" "${INSTALL_DIR}/xinugate"
run_priv cp -f "${TMP_DIR}/config.example.yaml" "${INSTALL_DIR}/config.example.yaml"
run_priv cp -f "${TMP_DIR}/LICENSE" "${INSTALL_DIR}/LICENSE"
run_priv cp -f "${TMP_DIR}/README.md" "${INSTALL_DIR}/README.md"

# 4. Generate config.yaml on first install (preserve existing config.yaml on upgrades)
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

if ! run_priv test -f "${INSTALL_DIR}/config.yaml"; then
  NEW_CONFIG=1
  MGMT_KEY="xg-admin-$(random_hex 12)"
  PROXY_KEY="xg-key-$(random_hex 16)"

  cat > "${TMP_DIR}/config.yaml.new" <<EOF
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
  run_priv install -m 0600 "${TMP_DIR}/config.yaml.new" "${INSTALL_DIR}/config.yaml"
fi

# 5. Configure and start systemd service when running with root/sudo privileges
SERVICE_ACTIVE=0
if [ "${IS_ROOT}" -eq 1 ] && [ "${NO_SERVICE}" -eq 0 ] && command -v systemctl >/dev/null 2>&1; then
  echo "==> Configuring systemd service (xinugate.service)..."
  cat > "${TMP_DIR}/xinugate.service" <<EOF
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

  run_priv install -m 0644 "${TMP_DIR}/xinugate.service" /etc/systemd/system/xinugate.service
  run_priv systemctl daemon-reload
  run_priv systemctl enable --now xinugate.service
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
if [ "${NEW_CONFIG}" -eq 1 ]; then
  echo "  [SAVE THESE KEYS]"
  echo "  Web UI Login Key  : ${MGMT_KEY}"
  echo "  Client API Key    : ${PROXY_KEY}"
  echo ""
else
  echo "  Existing config.yaml preserved at ${INSTALL_DIR}/config.yaml"
  echo ""
fi
if [ "${SERVICE_ACTIVE}" -eq 1 ]; then
  echo "  Service status    : xinugate.service is RUNNING"
  echo "  Manage service    : systemctl status|restart|stop xinugate"
  echo "  View live logs    : journalctl -u xinugate -f"
else
  echo "  Start manually    :"
  echo "    cd ${INSTALL_DIR} && ./xinugate --config config.yaml -sqlitestore"
fi
echo "=================================================================="
