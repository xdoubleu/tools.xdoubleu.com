#!/usr/bin/env bash
# Exercises scripts/resolve_deploy_version.sh against a stubbed GitHub API and
# Kamal, and a throwaway git origin that the workspace shallow-clones the way
# actions/checkout does. Commit 2 changes the deploy config; 1-6 are linear.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/scripts/resolve_deploy_version.sh"

fail=0
pass() { printf 'PASS: %s\n' "$1"; }
failn() { printf 'FAIL: %s\n' "$1"; fail=1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
FX="$WORK/fx"
mkdir -p "$WORK/bin" "$FX"

# --- origin repo: S[1]..S[6], S[2] changes config/deploy.api.yml -----------
ORIGIN="$WORK/origin"
git init -q "$ORIGIN"
git -C "$ORIGIN" config user.email t@example.com
git -C "$ORIGIN" config user.name t
git -C "$ORIGIN" config uploadpack.allowAnySHA1InWant true
mkdir -p "$ORIGIN/config" "$ORIGIN/.kamal"
echo a > "$ORIGIN/config/deploy.api.yml"
echo s > "$ORIGIN/.kamal/secrets"
echo g > "$ORIGIN/Gemfile"
S=("")
for n in 1 2 3 4 5 6; do
  if [ "$n" = 2 ]; then echo b > "$ORIGIN/config/deploy.api.yml"; fi
  echo "$n" > "$ORIGIN/f$n"
  git -C "$ORIGIN" add -A
  git -C "$ORIGIN" commit -q -m "c$n"
  S+=("$(git -C "$ORIGIN" rev-parse HEAD)")
done

# --- stubs ------------------------------------------------------------
# gh api ENDPOINT [--jq EXPR] over the fixtures built by fx/fx_norun.
cat > "$WORK/bin/gh" <<'SH'
#!/usr/bin/env bash
[ "$1" = api ] || exit 2
ep=$2
shift 2
if [ -e "$FX/gh.fail" ]; then echo "gh: HTTP 500" >&2; exit 1; fi
case $ep in
  repos/*/commits\?sha=main*)
    body=$(jq -R '{sha: .}' "$FX/commits.txt" | jq -s .) ;;
  repos/*/actions/workflows/main.yml/runs\?branch=main\&event=push*)
    body=$(jq -R 'split(" ") | {head_sha: .[0], id: (.[1] | tonumber)}' "$FX/runs.txt" | jq -s '{workflow_runs: .}') ;;
  repos/*/actions/runs/*/jobs\?filter=latest*)
    id=${ep#*/runs/}
    body=$(cat "$FX/jobs-${id%%/*}.json") ;;
  *) echo "unexpected endpoint: $ep" >&2; exit 2 ;;
esac
if [ "${1:-}" = --jq ]; then jq -r "$2" <<<"$body"; else echo "$body"; fi
SH

# bundle exec kamal app version -q -c config/deploy.SVC.yml: with -q Kamal
# prints the live version and a blank line.
cat > "$WORK/bin/bundle" <<'SH'
#!/usr/bin/env bash
cfg=${*: -1}
svc=${cfg#config/deploy.}
svc=${svc%.yml}
if [ -e "$FX/live-$svc.fail" ]; then echo "ERROR (Errno::ECONNREFUSED)"; exit 1; fi
if [ -e "$FX/live-$svc" ]; then cat "$FX/live-$svc"; fi
SH
chmod +x "$WORK/bin/gh" "$WORK/bin/bundle"

# --- fixture builders ---------------------------------------------------
fx_reset() { rm -rf "$FX"; mkdir -p "$FX"; : > "$FX/commits.txt"; : > "$FX/runs.txt"; }

# fx IDX MIG INFRA BUILD_API BUILD_GRAFANA: a main commit (call newest first)
# with a push run whose jobs ended as given: success|skipped|failure|cancelled,
# pending (not finished) or missing (not created yet).
fx() {
  local idx=$1 run=$((100 + $1))
  echo "${S[$idx]}" >> "$FX/commits.txt"
  echo "${S[$idx]} $run" >> "$FX/runs.txt"
  jq -n --arg mig "$2" --arg infra "$3" --arg bapi "$4" --arg bgraf "$5" '
    def job($name; $c):
      if $c == "missing" then empty
      else {name: $name,
            status: (if $c == "pending" then "in_progress" else "completed" end),
            conclusion: (if $c == "pending" then null else $c end)} end;
    {jobs: [
      {name: "changes", status: "completed", conclusion: "success"},
      job("Migration Versions on Main"; $mig),
      job("Apply Infra via OpenTofu"; $infra),
      job(if $bapi == "skipped" then "Build API Workflow" else "Build API Workflow / Build API" end; $bapi),
      job(if $bgraf == "skipped" then "Build Grafana Workflow" else "Build Grafana Workflow / Build Grafana" end; $bgraf),
      {name: "Deploy to Hetzner via Kamal", status: "queued", conclusion: null}
    ]}' > "$FX/jobs-$run.json"
}
# fx_norun IDX: a main commit whose push run doesn't exist (yet).
fx_norun() { echo "${S[$1]}" >> "$FX/commits.txt"; }
# live SERVICE IDX|none|unreadable|mixed|noisy
live() {
  rm -f "$FX/live-$1" "$FX/live-$1.fail"
  case $2 in
    none) printf '\n' > "$FX/live-$1" ;;
    unreadable) : > "$FX/live-$1.fail" ;;
    mixed) printf '%s\n\n%s\n\n' "${S[3]}" "${S[4]}" > "$FX/live-$1" ;;
    noisy) printf '  INFO connecting\n%s\n\nabc123\n' "${S[$3]}" > "$FX/live-$1" ;;
    *) printf '%s\n\n' "${S[$2]}" > "$FX/live-$1" ;;
  esac
}

# resolve OWN_IDX ATTEMPT SERVICE...: runs the script in a shallow checkout of
# S[OWN_IDX]; sets RC, and reads results with got/errs.
resolve() {
  local own=$1 attempt=$2
  shift 2
  rm -rf "$WORK/ws"
  mkdir "$WORK/ws"
  git -C "$WORK/ws" init -q
  git -C "$WORK/ws" remote add origin "file://$ORIGIN"
  git -C "$WORK/ws" fetch -q --no-tags --depth=1 origin "${S[$own]}"
  git -C "$WORK/ws" checkout -q --detach FETCH_HEAD
  : > "$WORK/out"
  (cd "$WORK/ws" && PATH="$WORK/bin:$PATH" FX="$FX" GH_TOKEN=x GITHUB_REPOSITORY=o/r \
    GITHUB_SHA="${S[$own]}" GITHUB_RUN_ATTEMPT="$attempt" GITHUB_OUTPUT="$WORK/out" \
    GITHUB_SERVER_URL=https://github.example "$SCRIPT" "$@") 2> "$WORK/err"
  RC=$?
}
got() { sed -n "s/^$1=//p" "$WORK/out"; }

# expect LABEL SERVICE IDX|skip: asserts a clean run resolved SERVICE as given.
expect() {
  local want=$3
  [ "$want" = skip ] || want=${S[$3]}
  if [ "$RC" = 0 ] && [ "$(got "$2")" = "$want" ]; then
    pass "$1"
  else
    failn "$1 (rc=$RC, $2=$(got "$2"), want $want; $(tr '\n' ' ' < "$WORK/err"))"
  fi
}

# --- the scenarios ---------------------------------------------------------

# 1. A lone push ships its own commit; grafana wasn't built anywhere.
fx_reset
fx 6 success skipped success skipped
live api none; live grafana none
resolve 6 1 api grafana
expect "lone push: api ships its own commit" api 6
expect "lone push: unbuilt grafana is skipped" grafana skip

# 2. The race: an older commit's job runs last, tip is built, live is older.
fx_reset
fx 6 success skipped success success
fx 5 success skipped success success
fx 4 success skipped success success
fx 3 success skipped success success
live api 3; live grafana 3
resolve 4 1 api grafana
expect "burst: the older job ships the tip (api)" api 6
expect "burst: the older job ships the tip (grafana)" grafana 6
if grep -q "shipping ${S[6]}" "$WORK/err"; then pass "burst: the cross-deploy is logged"; else failn "burst: the cross-deploy is logged"; fi

# 3. Tip still building: ship the newest built commit; the tip's own job follows.
fx_reset
fx 6 success missing pending pending
fx 5 pending missing pending pending
fx 4 success skipped success success
fx 3 success skipped success success
live api 3; live grafana 3
resolve 4 1 api grafana
expect "tip still building: ships the newest built (api)" api 4
expect "tip still building: ships the newest built (grafana)" grafana 4

# 4. Tip's grafana build failed: its deploy would never run, so don't ship it.
fx_reset
fx 6 success skipped success failure
fx 5 success skipped success success
live api 4; live grafana 4
resolve 5 1 api grafana
expect "failed build on the tip: api falls back to the previous commit" api 5
expect "failed build on the tip: grafana falls back too" grafana 5

# 5. infra-apply gates: pending, cancelled and failed keep the tip back.
for state in pending cancelled failure; do
  fx_reset
  fx 6 success "$state" success skipped
  fx 5 success skipped success skipped
  live api 4; live grafana none
  resolve 5 1 api
  expect "infra-apply $state on the tip: previous commit ships" api 5
done

# 6. migration-versions must have passed; no run at all is not deployable.
fx_reset
fx 6 failure skipped success skipped
fx 5 success skipped success skipped
live api 4
resolve 5 1 api
expect "migration-versions failed on the tip: previous commit ships" api 5
fx_reset
fx_norun 6
fx 5 success skipped success skipped
live api 4
resolve 5 1 api
expect "no run for the tip: previous commit ships" api 5

# 7. Never replace a newer live version, nor redeploy a live one.
fx_reset
fx 6 success skipped success success
fx 5 success skipped success success
fx 4 success skipped success success
live api 6; live grafana 6
resolve 4 1 api grafana
expect "tip already live: the older job skips (api)" api skip
expect "tip already live: the older job skips (grafana)" grafana skip
resolve 6 1 api
expect "live == own after a cross-deploy: the tip's own job skips" api skip
resolve 4 2 api
expect "re-run of an older job never downgrades" api skip
resolve 6 2 api grafana
expect "re-run of the live commit's own job redeploys it (api)" api 6
expect "re-run of the live commit's own job redeploys it (grafana)" grafana 6
live api 5
resolve 6 1 api
expect "live is older than the tip: the tip's own job ships it" api 6

# 8. Each service ships the newest commit that built it.
fx_reset
fx 6 success skipped skipped success
fx 5 success skipped success skipped
fx 4 success skipped success success
live api 4; live grafana 3
resolve 4 1 api grafana
expect "api skipped on the tip: ships the newest api build" api 5
expect "grafana built on the tip: ships the tip" grafana 6

# 9. Live version unknown or ambiguous: deploy rather than guess.
fx_reset
fx 6 success skipped success skipped
fx 5 success skipped success skipped
live api unreadable
resolve 5 1 api
expect "live version unreadable: ships the tip" api 6
if grep -q '::warning::' "$WORK/err"; then pass "live version unreadable: warns"; else failn "live version unreadable: warns"; fi
live api mixed
resolve 5 1 api
expect "live versions disagree: ships the tip" api 6
live api noisy 6
resolve 6 1 api
expect "log noise around the live version is ignored" api skip

# 10. Nothing deployable in the window.
fx_reset
fx 6 failure skipped success skipped
live api none
resolve 6 1 api
expect "nothing eligible: skip" api skip

# 11. Deploy-config guard: an older job must not ship a newer config change.
fx_reset
fx 6 success skipped success skipped
fx 5 success skipped success skipped
fx 3 success skipped success skipped
fx 1 success skipped success skipped
live api none
resolve 1 1 api
if [ "$RC" = 1 ] && [ -z "$(got api)" ] \
  && grep -q "::error::api: main has ${S[6]}" "$WORK/err" \
  && grep -q "github.example/o/r/actions/runs/106" "$WORK/err"; then
  pass "config changed since this run: fails and points at the tip's run"
else
  failn "config changed since this run: fails and points at the tip's run (rc=$RC; $(cat "$WORK/err"))"
fi
resolve 3 1 api
expect "unchanged config: the cross-deploy goes ahead" api 6

# 12. GitHub failures and bad usage fail the step instead of guessing.
touch "$FX/gh.fail"
resolve 6 1 api
if [ "$RC" != 0 ] && [ ! -s "$WORK/out" ]; then pass "GitHub API failure: fails, writes nothing"; else failn "GitHub API failure: fails, writes nothing (rc=$RC)"; fi
rm -f "$FX/gh.fail"
resolve 6 1 web
if [ "$RC" = 2 ]; then pass "unknown service: usage error"; else failn "unknown service: usage error (rc=$RC)"; fi

# 13. Replays the incident in both finish orders: every job runs, one at a
# time, and the live version is whatever the previous job shipped.
fx_reset
fx 6 success skipped success skipped
fx 5 success skipped success skipped
fx 4 success skipped success skipped
fx 3 success skipped success skipped
for order in "4 5 6" "6 5 4" "6 4" "5 4"; do
  live api 3
  for own in $order; do
    resolve "$own" 1 api
    v=$(got api)
    if [ "$v" != skip ] && [ -n "$v" ]; then printf '%s\n\n' "$v" > "$FX/live-api"; fi
  done
  if [ "$(tr -d '\n' < "$FX/live-api")" = "${S[6]}" ]; then
    pass "jobs run as $order: production ends on the tip"
  else
    failn "jobs run as $order: ends on $(tr -d '\n' < "$FX/live-api")"
  fi
done

# 14. The job names the script reads must exist in the workflows.
names_ok=1
for pair in \
  "main.yml|name: Migration Versions on Main" \
  "main.yml|name: Apply Infra via OpenTofu" \
  "main.yml|name: Build API Workflow" \
  "main.yml|name: Build Grafana Workflow" \
  "main.yml|name: Deploy to Hetzner via Kamal" \
  "build-api.yml|name: Build API" \
  "build-grafana.yml|name: Build Grafana"; do
  if ! grep -qxF "    ${pair#*|}" "$ROOT_DIR/.github/workflows/${pair%%|*}"; then
    names_ok=0
    failn "job name drifted from the script: ${pair#*|} in ${pair%%|*}"
  fi
done
for n in 'Migration Versions on Main' 'Apply Infra via OpenTofu' 'Build API Workflow' 'Build Grafana Workflow'; do
  grep -qF "'$n'" "$SCRIPT" || { names_ok=0; failn "script doesn't read job name: $n"; }
done
[ "$names_ok" = 1 ] && pass "job names match the workflows"

exit "$fail"
