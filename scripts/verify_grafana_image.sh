#!/usr/bin/env bash
# Boot the Grafana wrapper image and assert its baked-in provisioning
# actually loads (issue #1533). `make lint/grafana` only statically checks
# the dashboard JSON; a malformed provisioning YAML, a dashboard
# schemaVersion Grafana rejects, or a bad datasource block otherwise only
# shows up as a silent `level=error` line in Grafana's log after deploy.
#
# Run via `make grafana/verify`; also run by build-grafana.yml.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
image="tools-xdoubleu-com-grafana:verify"
container="grafana-verify-$$"
port=3999

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  docker rmi "$image" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> building $image"
docker build -q -f "$repo_root/infra/grafana.Dockerfile" -t "$image" "$repo_root" >/dev/null

echo "==> starting container"
docker run -d --name "$container" -p "$port:3000" \
  -e GF_SERVER_ROOT_URL="http://localhost:$port" \
  -e GF_LOG_LEVEL=info \
  -e GF_SMTP_FROM_ADDRESS="alerts@example.com" \
  -e NOTIFY_EMAIL_TO="alerts@example.com" \
  "$image" >/dev/null

base="http://localhost:$port"
echo "==> waiting for /api/health"
for _ in $(seq 1 30); do
  if curl -sf "$base/api/health" >/dev/null 2>&1; then break; fi
  sleep 1
done
curl -sf "$base/api/health" >/dev/null || { echo "FAIL: Grafana never became healthy"; docker logs "$container"; exit 1; }

fail=0

echo "==> checking for provisioning errors in the container log"
if docker logs "$container" 2>&1 | grep -E 'level=error.*(provision|dashboard|datasource)'; then
  echo "FAIL: provisioning error(s) in the Grafana log (above)"
  fail=1
fi

echo "==> checking the Prometheus datasource provisioned"
ds_uids=$(curl -sf -u admin:admin "$base/api/datasources" | python3 -c 'import json,sys; print("\n".join(d["uid"] for d in json.load(sys.stdin)))')
if ! grep -qx "prometheus" <<<"$ds_uids"; then
  echo "FAIL: datasource uid 'prometheus' not found (got: ${ds_uids:-none})"
  fail=1
fi

echo "==> checking every dashboard JSON provisioned"
want=$(cd "$repo_root/infra/grafana/dashboards" && for f in *.json; do
  python3 -c "import json;print(json.load(open('$f'))['uid'])"
done | sort)
got=$(curl -sf -u admin:admin "$base/api/search?type=dash-db" \
  | python3 -c 'import json,sys; print("\n".join(d["uid"] for d in json.load(sys.stdin)))' | sort)
missing=$(comm -23 <(echo "$want") <(echo "$got") || true)
if [ -n "$missing" ]; then
  echo "FAIL: dashboards missing from Grafana: $(echo "$missing" | tr '\n' ' ')"
  fail=1
fi

echo "==> checking the alerting contact point provisioned"
cp_names=$(curl -sf -u admin:admin "$base/api/v1/provisioning/contact-points" \
  | python3 -c 'import json,sys; print("\n".join(c["name"] for c in json.load(sys.stdin)))')
if ! grep -qx "email" <<<"$cp_names"; then
  echo "FAIL: contact point 'email' not found (got: ${cp_names:-none})"
  fail=1
fi

echo "==> checking the alert rules provisioned"
want_rules="APIDown FrontendP95High HostCPUHigh HostDiskHigh HostMemoryHigh IssueFailingPRs IssueMainCIRed IssueOrphanedStorage IssueSecurityAlerts IssueSentryUnresolved JobP95High PostgresDown R2UsageHigh RequestP95High"
got_rules=$(curl -sf -u admin:admin "$base/api/v1/provisioning/alert-rules" \
  | python3 -c 'import json,sys; print(" ".join(sorted(r["title"] for r in json.load(sys.stdin))))')
for rule in $want_rules; do
  if ! grep -qw "$rule" <<<"$got_rules"; then
    echo "FAIL: alert rule '$rule' not provisioned (got: ${got_rules:-none})"
    fail=1
  fi
done

if [ "$fail" -ne 0 ]; then
  echo "grafana image verification FAILED"
  exit 1
fi
rule_count=$(set -- $want_rules; echo $#)
echo "ok: datasource + $(grep -c . <<<"$got") dashboard(s) + $rule_count alert rule(s) + contact point provisioned, no provisioning errors"
