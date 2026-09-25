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

wait
exit 0
