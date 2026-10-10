#!/usr/bin/env bash
#
# Installs or upgrades HomeApps as a systemd service, the same way the other
# home apps install (and so HostMan can deploy it like any of them):
#
#   curl -fsSL https://raw.githubusercontent.com/chinmay28/homeapps/main/scripts/quickstart.sh | sudo bash
#
# and removes it again with:
#
#   curl -fsSL https://raw.githubusercontent.com/chinmay28/homeapps/main/scripts/quickstart.sh | sudo bash -s -- --uninstall
#
# HomeApps keeps no data of its own — the list of apps comes from HostMan — so
# an upgrade is just a binary swap: the new build is health-checked after it
# starts and the previous binary comes back if it doesn't answer.
#
# Environment overrides (remembered in /etc/homeapps.env across upgrades):
#   HOMEAPPS_PORT          port to listen on                 (default 8686)
#   HOMEAPPS_HOSTMAN       HostMan to read apps from         (default http://127.0.0.1:8899)
#   HOMEAPPS_HOSTMAN_PIN   HostMan's PIN, if it has one      (default none)
#   HOMEAPPS_REF           git branch, tag or commit         (default main)
#   HOMEAPPS_REPO          repository to build from
#
set -euo pipefail

REF="${HOMEAPPS_REF:-main}"
REPO="${HOMEAPPS_REPO:-https://github.com/chinmay28/homeapps.git}"
INSTALL_DIR="${HOMEAPPS_INSTALL_DIR:-/opt/homeapps}"
BUILD_DIR="$INSTALL_DIR/src"
BIN="$INSTALL_DIR/bin/homeapps"
ENV_FILE="${HOMEAPPS_ENV_FILE:-/etc/homeapps.env}"
UNIT="${HOMEAPPS_UNIT:-/etc/systemd/system/homeapps.service}"
SERVICE_USER="${HOMEAPPS_SERVICE_USER:-homeapps}"
GO_DIR="${HOMEAPPS_GO_DIR:-/usr/local/go}"
GO_VERSION=1.24.7

log()  { printf '\033[1;35m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m!!\033[0m %s\n' "$*" >&2; exit 1; }
# Never fail silently: under set -e a failing command would otherwise end the
# run with no word of what broke, which is all HostMan's log could show.
trap 'rc=$?; printf "\033[1;31m!!\033[0m failed (exit %s) at line %s: %s\n" "$rc" "$LINENO" "$BASH_COMMAND" >&2' ERR

[ "$(id -u)" -eq 0 ] || die "Run this with sudo."

case "${1:-}" in
  --uninstall)
    log "Stopping and removing the HomeApps service"
    systemctl disable --now homeapps.service 2>/dev/null || true
    rm -f "$UNIT" "$ENV_FILE"
    systemctl daemon-reload
    rm -rf "$INSTALL_DIR"
    userdel "$SERVICE_USER" 2>/dev/null || true
    log "Removed. HomeApps kept no data, so there is nothing else to clean up."
    exit 0 ;;
  "") ;;
  *) die "Unknown option: $1 (only --uninstall is supported)" ;;
esac

command -v systemctl >/dev/null || die "This installer needs systemd."
command -v apt-get >/dev/null || die "This installer expects a Debian or Ubuntu system."

# ------------------------------------------------------------------ settings
# Values given now win; otherwise keep what the last install wrote; otherwise
# the defaults. The file is root-only because it may hold HostMan's PIN.
# Always succeeds: on a first install there is no file, and a failing command
# substitution inside an assignment would end the script under set -e.
saved() {
  if [ -f "$ENV_FILE" ]; then sed -n "s/^$1=//p" "$ENV_FILE" | tail -n 1; fi
  return 0
}
PORT="${HOMEAPPS_PORT:-$(saved PORT)}";                 PORT="${PORT:-8686}"
HOSTMAN="${HOMEAPPS_HOSTMAN:-$(saved HOMEAPPS_HOSTMAN)}"; HOSTMAN="${HOSTMAN:-http://127.0.0.1:8899}"
PIN="${HOMEAPPS_HOSTMAN_PIN:-$(saved HOMEAPPS_HOSTMAN_PIN)}"
case "$PORT" in ''|*[!0-9]*) die "HOMEAPPS_PORT must be a number, got '$PORT'." ;; esac

# ------------------------------------------------------------- build tools
missing=()
for pkg in git curl ca-certificates; do
  dpkg -s "$pkg" >/dev/null 2>&1 || missing+=("$pkg")
done
if [ "${#missing[@]}" -gt 0 ]; then
  log "Installing ${missing[*]}"
  apt-get update -qq
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "${missing[@]}" >/dev/null
fi

case "$(uname -m)" in
  x86_64|amd64)  GO_ARCH=amd64 ;;
  aarch64|arm64) GO_ARCH=arm64 ;;
  armv6l|armv7l) GO_ARCH=armv6l ;;
  *) die "Unsupported architecture: $(uname -m)" ;;
esac

export PATH="$GO_DIR/bin:$PATH"
have="$(go env GOVERSION 2>/dev/null | sed 's/^go//' || true)"
if [ -z "$have" ] || [ "$(printf '%s\n%s\n' "$GO_VERSION" "$have" | sort -V | head -n 1)" != "$GO_VERSION" ]; then
  log "Installing Go $GO_VERSION (build-time only)"
  tmp="$(mktemp)"
  curl -fsSL --retry 3 -o "$tmp" "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" ||
    die "Could not download Go $GO_VERSION. Check the network and try again."
  # Unpack beside the old toolchain and swap, so a failed download never
  # leaves the machine without the Go it had.
  rm -rf "$GO_DIR.new" && mkdir -p "$GO_DIR.new"
  tar -C "$GO_DIR.new" --strip-components=1 -xzf "$tmp" || die "The Go download is not a valid archive."
  rm -f "$tmp"
  [ -x "$GO_DIR.new/bin/go" ] || die "The Go archive has no go binary in it."
  rm -rf "$GO_DIR.old"; [ -d "$GO_DIR" ] && mv "$GO_DIR" "$GO_DIR.old"
  mv "$GO_DIR.new" "$GO_DIR"; rm -rf "$GO_DIR.old"
fi

# ------------------------------------------------------------------- source
# --filter=blob:none, never --depth 1: the version's patch number is the
# commit count, which a shallow clone gets wrong (scripts/version.sh).
if [ -d "$BUILD_DIR/.git" ]; then
  log "Fetching $REF"
  git -C "$BUILD_DIR" fetch --quiet --tags origin
else
  log "Cloning $REPO"
  rm -rf "$BUILD_DIR"; mkdir -p "$INSTALL_DIR"
  git clone --quiet --filter=blob:none "$REPO" "$BUILD_DIR"
fi
if git -C "$BUILD_DIR" rev-parse --verify --quiet "origin/$REF" >/dev/null; then
  git -C "$BUILD_DIR" checkout --quiet --force -B homeapps-build "origin/$REF"
else
  git -C "$BUILD_DIR" checkout --quiet --force "$REF"
fi

log "Building"
VERSION="$("$BUILD_DIR/scripts/version.sh")"
PATCH="$("$BUILD_DIR/scripts/version.sh" --patch)"
mkdir -p "$INSTALL_DIR/bin"
(cd "$BUILD_DIR/server" && CGO_ENABLED=0 GOFLAGS=-mod=readonly go build -trimpath \
  -ldflags "-s -w -X github.com/chinmay28/homeapps/server/internal/version.Patch=$PATCH" \
  -o "$BIN.new" ./cmd/homeapps)

# ---------------------------------------------------------------- service
id "$SERVICE_USER" >/dev/null 2>&1 ||
  useradd --system --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin "$SERVICE_USER"

umask 077
cat > "$ENV_FILE" <<ENV
PORT=$PORT
HOMEAPPS_HOSTMAN=$HOSTMAN
HOMEAPPS_HOSTMAN_PIN=$PIN
ENV
umask 022

cat > "$UNIT" <<UNIT
[Unit]
Description=HomeApps launcher
After=network-online.target
Wants=network-online.target

[Service]
User=$SERVICE_USER
EnvironmentFile=$ENV_FILE
ExecStart=$BIN serve
Restart=on-failure
RestartSec=2
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes

[Install]
WantedBy=multi-user.target
UNIT

[ -f "$BIN" ] && cp -f "$BIN" "$BIN.prev"
mv -f "$BIN.new" "$BIN"
systemctl daemon-reload
systemctl enable --quiet homeapps.service
systemctl restart homeapps.service

# --------------------------------------------------------- health & rollback
healthy=0
for _ in $(seq 1 20); do
  if curl -fsS --max-time 2 "http://127.0.0.1:$PORT/api/health" >/dev/null 2>&1; then healthy=1; break; fi
  sleep 1
done
if [ "$healthy" = 0 ]; then
  warn "HomeApps $VERSION did not answer on port $PORT."
  if [ -f "$BIN.prev" ]; then
    warn "Rolling back to the previous version"
    mv -f "$BIN.prev" "$BIN"
    systemctl restart homeapps.service
    die "Upgrade failed and was rolled back. See: journalctl -u homeapps -n 50"
  fi
  die "HomeApps did not start. See: journalctl -u homeapps -n 50"
fi
rm -f "$BIN.prev"

host="$(hostname -I 2>/dev/null | awk '{print $1}')"
echo
log "HomeApps $VERSION is running: http://${host:-localhost}:$PORT"
log "Reading apps from HostMan at $HOSTMAN"
if ! curl -fsS --max-time 5 "http://127.0.0.1:$PORT/api/catalog" >/dev/null 2>&1; then
  warn "HomeApps cannot read HostMan at $HOSTMAN yet. Re-run with HOMEAPPS_HOSTMAN=http://<host>:8899"
  warn "(and HOMEAPPS_HOSTMAN_PIN=… if HostMan has a PIN)."
fi
