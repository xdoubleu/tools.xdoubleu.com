#!/usr/bin/env bash
# Probes the Mac sandbox the factory runner would use (docs/adr-0043): Apple
# `container` on a host-only network. Run it on the Mac that will host jobs
# (macOS 26, Apple silicon, the `container` CLI); a cloud agent can't. It was
# written from the CLI docs without a Mac, so a flag the CLI rejects is the
# script's bug. It prints PASS/FAIL per check; paste the output into #2704.
set -uo pipefail

IMAGE=${PROBE_IMAGE:-docker.io/curlimages/curl:latest} # non-root (uid 100), has curl and sh
NET=factory-probe-net
WORK=$(mktemp -d)
PROXY_PORT=${PROBE_PROXY_PORT:-18080}
fails=0

pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; fails=$((fails + 1)); }
note() { printf 'NOTE: %s\n' "$1"; }

cleanup() {
  [ -n "${SRV_PID:-}" ] && kill "$SRV_PID" 2>/dev/null
  container network delete "$NET" >/dev/null 2>&1
  rm -rf "$WORK"
}
trap cleanup EXIT

# run_in CMD...: run sh -c CMD in a throwaway VM on the probe network, with the
# job clone writable, a second mount read-only and the root filesystem read-only.
run_in() {
  container run --rm --network "$NET" --read-only --tmpfs /tmp \
    --cap-drop ALL --cpus 2 --memory 2G \
    --mount "type=bind,source=$WORK/clone,target=/work" \
    --mount "type=bind,source=$WORK/ro,target=/ro,readonly" \
    --entrypoint sh "$IMAGE" -c "$1"
}

# 0. Preflight
[ "$(uname -s)" = Darwin ] || { echo "not macOS"; exit 2; }
[ "$(uname -m)" = arm64 ] || { echo "not Apple silicon"; exit 2; }
note "macOS $(sw_vers -productVersion)"
command -v container >/dev/null || { echo "no container CLI"; exit 2; }
note "$(container --version 2>&1 | head -n 1)"
container system start >/dev/null 2>&1 || note "container system start failed or already running"

mkdir -p "$WORK/clone" "$WORK/ro"
echo secret >"$WORK/ro/file"
chmod 777 "$WORK/clone"

# 1. A host-only network, and an image pulled while the host still has internet.
container image pull "$IMAGE" >/dev/null 2>&1 && pass "image pulled" || fail "image pull"
container network create --internal "$NET" >/dev/null 2>&1 \
  && pass "network create --internal" || fail "network create --internal"

# 2. The gateway the VM sees: the host side of the network, where the proxy lives.
GATEWAY=$(run_in "ip route | awk '/default/ {print \$3}'" 2>/dev/null | head -n 1)
if [ -n "$GATEWAY" ]; then
  pass "VM has a default route via $GATEWAY"
else
  fail "no default gateway inside the VM (the proxy needs one to be reachable)"
fi

# 3. Egress: no direct route out. Both must fail.
run_in "curl -sS -m 5 -o /dev/null https://example.com" >/dev/null 2>&1 \
  && fail "VM reached example.com directly" || pass "direct HTTPS to example.com fails"
run_in "curl -sS -m 5 -o /dev/null http://1.1.1.1" >/dev/null 2>&1 \
  && fail "VM reached 1.1.1.1 directly" || pass "direct HTTP to 1.1.1.1 fails"

# 4. The host proxy path. A stand-in that records the request, bound to the
# gateway only. A dummy bearer token must arrive, and that is where the real
# proxy would swap it.
if [ -n "$GATEWAY" ]; then
  cat >"$WORK/stub.py" <<PY
import http.server, sys
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        sys.stderr.write("AUTH=%s\n" % self.headers.get("Authorization"))
        self.send_response(200); self.end_headers(); self.wfile.write(b"ok")
    def log_message(self, *a): pass
http.server.HTTPServer(("$GATEWAY", $PROXY_PORT), H).serve_forever()
PY
  python3 "$WORK/stub.py" 2>"$WORK/stub.log" &
  SRV_PID=$!
  sleep 1
  out=$(run_in "curl -sS -m 5 -H 'Authorization: Bearer dummy' http://$GATEWAY:$PROXY_PORT/" 2>&1)
  [ "$out" = ok ] && pass "VM reaches the host stub on $GATEWAY:$PROXY_PORT" \
    || fail "VM can't reach the host stub ($out)"
  grep -q 'AUTH=Bearer dummy' "$WORK/stub.log" \
    && pass "the stub saw the dummy Authorization header" \
    || fail "the stub never saw the header"
  # Anything else listening on the host must not be reachable on this network.
  note "port 22 on the gateway: $(run_in "nc -z -w 2 $GATEWAY 22 && echo OPEN || echo closed" 2>&1 | tail -n 1) (want closed)"
fi

# 5. Isolation: no home directory, no Keychain, a read-only root, a read-only mount.
run_in "ls /Users 2>/dev/null | head -n 1" | grep -q . \
  && fail "the VM sees /Users" || pass "the VM has no /Users"
run_in "touch /x" >/dev/null 2>&1 && fail "root filesystem is writable" \
  || pass "root filesystem is read-only"
run_in "echo hi >/ro/new" >/dev/null 2>&1 && fail "read-only mount is writable" \
  || pass "read-only mount refuses writes"
run_in "echo hi >/work/new && id -u" >"$WORK/id.out" 2>&1 \
  && pass "job clone is writable as uid $(tail -n 1 "$WORK/id.out") (want non-root)" \
  || fail "job clone not writable as the non-root user"
[ -f "$WORK/clone/new" ] && pass "the write reached the host clone" \
  || fail "the write did not reach the host clone"
note "file owner on the host: $(stat -f '%Su:%Sg' "$WORK/clone/new" 2>/dev/null)"

# 6. Start-up time (create, run a no-op, destroy), five runs.
start=$(date +%s)
for _ in 1 2 3 4 5; do run_in true >/dev/null 2>&1; done
note "five create+run+destroy cycles took $(($(date +%s) - start))s"

# 7. Destroyed afterwards.
left=$(container ls -a 2>/dev/null | grep -c "$IMAGE" || true)
[ "$left" = 0 ] && pass "no container left behind (--rm)" \
  || fail "$left container(s) left behind"

cat <<'TXT'

Manual checks (can't be scripted here):
  1. Claude on the subscription: inside the VM, with ANTHROPIC_BASE_URL pointing at
     the host proxy and a dummy CLAUDE_CODE_OAUTH_TOKEN, does `claude -p "say ok"`
     work once the proxy swaps the header? And with the real token injected into the
     VM and only the proxy reachable?
  2. Plugins: in a fresh VM, does `claude -p` load this repo's marketplace plugins
     and .claude/skills/?
  3. Sleep: start `caffeinate -is -w <pid>` around the runner, close the lid on AC
     power with an external display, open it: did the heartbeat's long-poll recover?
  4. `make test` (needs Postgres) inside the image.
TXT

printf '\n%s check(s) failed\n' "$fails"
[ "$fails" = 0 ]
