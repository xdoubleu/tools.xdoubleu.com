#!/bin/bash
# Bootstraps a Claude Code on the web session so the documented lint/test
# commands work. No-op outside a remote session; idempotent.
set -uo pipefail

if [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0
fi

cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0

# --- 1. Go-installed tools first on PATH so pinned versions win.
GOBIN="$(go env GOPATH 2>/dev/null)/bin"
if [ -n "${CLAUDE_ENV_FILE:-}" ]; then
  echo "export PATH=\"$GOBIN:\$PATH\"" >> "$CLAUDE_ENV_FILE"
fi
export PATH="$GOBIN:$PATH"

# --- 2. Install the pinned lint/proto tools.
(cd api && make tools/lint tools/proto/local) >/tmp/claude-bootstrap-tools.log 2>&1 &

# --- 3. No Docker daemon: point the API at a local Postgres cluster.
if ! docker info >/dev/null 2>&1; then
  PG_VERSION="$(ls /etc/postgresql 2>/dev/null | sort -V | tail -1)"
  if [ -n "$PG_VERSION" ]; then
    HBA="/etc/postgresql/$PG_VERSION/main/pg_hba.conf"
    # Match DB_DSN's passwordless default (docker-compose's trust auth).
    if [ -f "$HBA" ] && ! grep -q "^host.*all.*all.*127.0.0.1/32.*trust" "$HBA"; then
      sed -i 's/^\(host\s\+all\s\+all\s\+127\.0\.0\.1\/32\s\+\)scram-sha-256/\1trust/' "$HBA"
      sed -i 's/^\(host\s\+all\s\+all\s\+::1\/128\s\+\)scram-sha-256/\1trust/' "$HBA"
    fi
    pg_ctlcluster "$PG_VERSION" main start 2>/dev/null || true
    for _ in $(seq 1 20); do
      pg_isready -h 127.0.0.1 -p 5432 >/dev/null 2>&1 && break
      sleep 0.5
    done
  fi
fi

# --- 4. Install web/node_modules if missing. A mismatched system Node
# rewrites package-lock.json, so discard that drift.
if [ -d web ] && [ ! -d web/node_modules ]; then
  (
    cd web && npm install >/tmp/claude-bootstrap-npm.log 2>&1
    if ! git diff --quiet -- package-lock.json; then
      echo "session-start: discarding package-lock.json drift from a Node version mismatch" >>/tmp/claude-bootstrap-npm.log
      git checkout -- package-lock.json
    fi
  ) &
fi

# --- 5. GOTOOLCHAIN=auto's per-module toolchain download (fetched here
# because this container's preinstalled Go is older than api/go.mod's `go`
# directive) ships without `go tool covdata` — an open upstream bug
# (golang/go#75031, targeted for Go 1.27) — which makes `make
# test/cov/report` exit 1 even though every individual test passes. The
# real fix (a full official release, as CI's actions/setup-go installs) is
# unreachable here: this environment's network policy blocks
# dl.google.com. Work around it by building covdata from the source tree
# the partial module download does ship, with that same module's own `go`
# binary, dropping the result into the module's own pkg/tool dir. See
# api/AGENTS.md's Testing Notes for the full writeup.
(
  cd api || exit 0
  SWITCHED_GOROOT="$(go env GOROOT 2>/dev/null)"
  GOOSARCH="$(go env GOOS 2>/dev/null)_$(go env GOARCH 2>/dev/null)"
  COVDATA_BIN="$SWITCHED_GOROOT/pkg/tool/$GOOSARCH/covdata"
  if [ -n "$SWITCHED_GOROOT" ] && [ ! -x "$COVDATA_BIN" ] &&
    [ -d "$SWITCHED_GOROOT/src/cmd/covdata" ] && [ -x "$SWITCHED_GOROOT/bin/go" ]; then
    chmod -R u+w "$SWITCHED_GOROOT" 2>/dev/null
    (
      cd "$SWITCHED_GOROOT/src/cmd/covdata" &&
        GOROOT="$SWITCHED_GOROOT" GOTOOLCHAIN=local "$SWITCHED_GOROOT/bin/go" build -o "$COVDATA_BIN" .
    ) >/tmp/claude-bootstrap-covdata.log 2>&1
    chmod -R a-w "$SWITCHED_GOROOT" 2>/dev/null
  fi
) &

wait
exit 0
