#!/usr/bin/env bash
#
# Runs scripts/quickstart.sh for real against this checkout: a fresh install,
# an upgrade that keeps the saved settings, a failed upgrade that rolls back,
# and an uninstall. Only the system-wide pieces are stubbed (systemctl starts
# the built binary itself, apt-get/dpkg/useradd do nothing) and every path the
# installer writes is moved into a temporary directory.
#
#   sudo scripts/test-quickstart.sh       # needs root, git and Go 1.24+
#
# This exists because a first install once failed silently on a line no other
# test reached; the only check that counts for an installer is running it.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
[ "$(id -u)" -eq 0 ] || { echo "run as root (the installer refuses otherwise)" >&2; exit 1; }

T="$(mktemp -d)"
PORT=18686
STUBS="$T/stubs"
mkdir -p "$STUBS"
cleanup() {
  [ -f "$T/pid" ] && kill "$(cat "$T/pid")" 2>/dev/null || true
  rm -rf "$T"
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok - $*"; }

# systemctl: "restart" runs the unit's ExecStart with its EnvironmentFile, the
# way systemd would; "disable --now" stops it. A flag file makes the next
# restart start nothing, which is how a broken upgrade is simulated.
cat > "$STUBS/systemctl" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
stop() { [ -f "$T/pid" ] && kill "$(cat "$T/pid")" 2>/dev/null || true; rm -f "$T/pid"; sleep 0.3; }
case "$1" in
  restart)
    stop
    if [ -f "$T/fail-next-start" ]; then rm -f "$T/fail-next-start"; exit 0; fi
    exec_start="$(sed -n 's/^ExecStart=//p' "$HOMEAPPS_UNIT")"
    env_file="$(sed -n 's/^EnvironmentFile=//p' "$HOMEAPPS_UNIT")"
    if grep -qx 'StateDirectory=homeapps' "$HOMEAPPS_UNIT"; then
      mkdir -p "$HOMEAPPS_STATE_DIR"
      export STATE_DIRECTORY="$HOMEAPPS_STATE_DIR"
    fi
    (set -a; . "$env_file"; set +a; exec $exec_start) > "$T/service.log" 2>&1 &
    echo $! > "$T/pid" ;;
  disable) stop ;;
  *) ;; # daemon-reload, enable
esac
STUB
for cmd in apt-get dpkg useradd userdel; do printf '#!/bin/sh\nexit 0\n' > "$STUBS/$cmd"; done
chmod +x "$STUBS"/*

export T
# Root reading a checkout owned by someone else (CI's runner user): let git
# trust it for this run only, without touching any git config file.
export GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=safe.directory GIT_CONFIG_VALUE_0='*'
export PATH="$STUBS:$PATH"
export HOMEAPPS_INSTALL_DIR="$T/opt"
export HOMEAPPS_ENV_FILE="$T/homeapps.env"
export HOMEAPPS_UNIT="$T/homeapps.service"
export HOMEAPPS_SERVICE_USER=root
export HOMEAPPS_STATE_DIR="$T/state"
export HOMEAPPS_GO_DIR="$(go env GOROOT)"
export HOMEAPPS_REPO="file://$ROOT"
export HOMEAPPS_REF="$(git -C "$ROOT" rev-parse HEAD)"

health() { curl -fsS --max-time 2 "http://127.0.0.1:$1/api/health" 2>/dev/null || true; }

# 1. Fresh install: no settings file yet.
HOMEAPPS_PORT=$PORT HOMEAPPS_HOSTMAN=http://127.0.0.1:1 bash "$ROOT/scripts/quickstart.sh" > "$T/run1.log" 2>&1 ||
  { cat "$T/run1.log"; fail "fresh install exited non-zero"; }
health $PORT | grep -q '"app":"homeapps"' || { cat "$T/run1.log" "$T/service.log"; fail "not serving after a fresh install"; }
[ "$(stat -c %a "$HOMEAPPS_ENV_FILE")" = 600 ] || fail "settings file is not root-only (it can hold HostMan's PIN)"
grep -qx "HOMEAPPS_HOSTMAN=http://127.0.0.1:1" "$HOMEAPPS_ENV_FILE" || fail "HostMan address not saved"
grep -q "warn\|cannot read HostMan" "$T/run1.log" || fail "an unreachable HostMan should be reported at the end"
pass "fresh install serves on :$PORT and saves its settings"

# 2. Upgrade with nothing set: the saved port, HostMan address and custom
#    apps are kept.
curl -fsS -X POST -H 'Content-Type: application/json' -d '{"name":"Router","url":"http://192.168.1.1"}' \
  "http://127.0.0.1:$PORT/api/custom-apps" >/dev/null || fail "could not add a custom app"
[ -f "$HOMEAPPS_STATE_DIR/custom-apps.json" ] || fail "custom apps not saved in the unit's StateDirectory"
bash "$ROOT/scripts/quickstart.sh" > "$T/run2.log" 2>&1 || { cat "$T/run2.log"; fail "upgrade exited non-zero"; }
health $PORT | grep -q '"app":"homeapps"' || fail "not serving on the saved port after an upgrade"
grep -qx "HOMEAPPS_HOSTMAN=http://127.0.0.1:1" "$HOMEAPPS_ENV_FILE" || fail "upgrade forgot the HostMan address"
curl -fsS "http://127.0.0.1:$PORT/api/custom-apps" | grep -q '"name":"Router"' || fail "upgrade lost the custom apps"
pass "upgrade keeps the saved settings and custom apps"

# 3. A new build that never answers is rolled back to the previous binary.
touch "$T/fail-next-start"
if bash "$ROOT/scripts/quickstart.sh" > "$T/run3.log" 2>&1; then fail "a failed upgrade reported success"; fi
grep -q "rolled back" "$T/run3.log" || { cat "$T/run3.log"; fail "no rollback message"; }
health $PORT | grep -q '"app":"homeapps"' || fail "previous version not running after the rollback"
pass "failed upgrade rolls back and keeps serving"

# 4. Failures say where: the ERR trap names the line, never a silent exit 1.
if HOMEAPPS_PORT=notaport bash "$ROOT/scripts/quickstart.sh" > "$T/run4.log" 2>&1; then fail "a bad port was accepted"; fi
grep -q "must be a number" "$T/run4.log" || { cat "$T/run4.log"; fail "a bad setting failed without saying why"; }
pass "a bad setting is reported, not silent"

# 5. Uninstall stops the service and removes what it installed.
bash "$ROOT/scripts/quickstart.sh" --uninstall > "$T/run5.log" 2>&1 || { cat "$T/run5.log"; fail "uninstall exited non-zero"; }
[ -z "$(health $PORT)" ] || fail "still serving after uninstall"
[ ! -e "$HOMEAPPS_INSTALL_DIR" ] && [ ! -e "$HOMEAPPS_UNIT" ] && [ ! -e "$HOMEAPPS_ENV_FILE" ] && [ ! -e "$HOMEAPPS_STATE_DIR" ] || fail "uninstall left files behind"
pass "uninstall stops the service and removes its files"
