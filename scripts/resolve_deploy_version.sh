#!/usr/bin/env bash
# Picks what main.yml's deploy-kamal ships per Kamal service: the newest main
# commit whose images are built and whose deploy `needs` passed, or "skip" when
# that or a newer one is already live. The concurrency group replaces its
# pending job in build-finish order, so the last job can belong to an older
# commit; this makes it ship main's tip anyway.
# A live version is still redeployed when what Kamal applies (deploy config,
# secrets, the deploy step's env) differs from the hash labelled on the live
# container, or when FORCE_DEPLOY=true.

# usage: resolve_deploy_version.sh api|grafana...
# env: GH_TOKEN GITHUB_REPOSITORY GITHUB_SHA KAMAL_SERVER_IP
#      [GITHUB_RUN_ATTEMPT GITHUB_OUTPUT FORCE_DEPLOY]
# Writes <service>=<sha|skip> and <service>_config_hash=<hash> to $GITHUB_OUTPUT
# (stdout when unset). The deploy step labels its containers with the hash
# (config/deploy.*.yml `labels`).
set -euo pipefail
shopt -s inherit_errexit

WINDOW=${DEPLOY_WINDOW:-15}

log() { echo "$*" >&2; }

# gh_api ARGS...: `gh api` retried on failure; GitHub's API sheds load with
# transient 5xx, and one failed call would fail the whole deploy.
gh_api() {
  local try out
  for try in 1 2 3 4; do
    if out=$(gh api "$@"); then
      printf '%s\n' "$out"
      return
    fi
    [[ $try -lt 4 ]] || break
    log "gh api $1 failed (attempt $try); retrying"
    sleep "$((${GH_RETRY_SLEEP:-5} * try))"
  done
  return 1
}

# conclusion NAME JOBS: a job's conclusion; "pending" until it has one, which
# includes jobs GitHub hasn't created yet (they appear once their needs end).
# Reusable-workflow jobs are listed as "<caller> / <job>", skipped ones as
# "<caller>" alone. These names must match main.yml (the test checks).
conclusion() {
  jq -r --arg n "$1" '
    [.jobs[] | select(.name == $n or (.name | startswith($n + " / ")))][0]
    | .conclusion // "pending"' <<<"$2"
}

# candidates: one line per main commit in the window, newest first:
#   <sha> <run id|-> <ok|no: deploy-kamal's needs passed> <api built> <grafana built>
candidates() {
  local runs sha run jobs gates mig infra bapi bgraf
  runs=$(gh_api "repos/$GITHUB_REPOSITORY/actions/workflows/main.yml/runs?branch=main&event=push&per_page=$((WINDOW * 2))" \
    --jq '[.workflow_runs[] | {sha: .head_sha, id: .id}]')
  while read -r sha; do
    run=$(jq -r --arg s "$sha" '[.[] | select(.sha == $s)][0].id // empty' <<<"$runs")
    if [[ -z $run ]]; then
      echo "$sha - no no no"
      continue
    fi
    jobs=$(gh_api "repos/$GITHUB_REPOSITORY/actions/runs/$run/jobs?filter=latest&per_page=100")
    mig=$(conclusion 'Migration Versions on Main' "$jobs")
    infra=$(conclusion 'Apply Infra via OpenTofu' "$jobs")
    bapi=$(conclusion 'Build API Workflow' "$jobs")
    bgraf=$(conclusion 'Build Grafana Workflow' "$jobs")
    gates=no
    if [[ $mig == success && $infra =~ ^(success|skipped)$ &&
      $bapi =~ ^(success|skipped)$ && $bgraf =~ ^(success|skipped)$ ]]; then
      gates=ok
    fi
    echo "$sha $run $gates $([[ $bapi == success ]] && echo yes || echo no) $([[ $bgraf == success ]] && echo yes || echo no)"
  done < <(gh_api "repos/$GITHUB_REPOSITORY/commits?sha=main&per_page=$WINDOW" --jq '.[].sha')
}

# decide SERVICE DEPLOYED OWN ATTEMPT < candidates: the sha to deploy or "skip".
# Walking newest first, reaching the live version before an eligible commit
# means nothing newer is deployable. A re-run (attempt > 1) of the live
# commit's own job still redeploys it, e.g. to pick up rotated secrets.
decide() {
  local svc=$1 deployed=$2 own=$3 attempt=$4 sha gates api grafana built
  while read -r sha _ gates api grafana; do
    built=$api
    [[ $svc == grafana ]] && built=$grafana
    if [[ $sha == "$deployed" ]]; then
      if [[ $sha == "$own" && $attempt -gt 1 && $gates == ok && $built == yes ]]; then
        echo "$sha"
      else
        log "::notice::$svc: $sha is live; nothing newer to deploy"
        echo skip
      fi
      return
    fi
    if [[ $gates == ok && $built == yes ]]; then
      echo "$sha"
      return
    fi
  done
  log "::notice::$svc: no built commit among main's last $WINDOW to deploy"
  echo skip
}

# The one Kamal version live for SERVICE, or nothing when unknown or mixed.
deployed_version() {
  local out versions
  if ! out=$(bundle exec kamal app version -q -c "config/deploy.$1.yml" 2>/dev/null); then
    log "::warning::Could not read the live $1 version; treating it as unknown"
    return
  fi
  mapfile -t versions < <(grep -E '^[0-9a-f]{40}$' <<<"$out" | sort -u || true)
  if [[ ${#versions[@]} -eq 1 ]]; then
    echo "${versions[0]}"
  else
    log "::warning::Live $1 version is ${#versions[@]} values; treating it as unknown"
  fi
}

# config_hash SERVICE: a hash of everything Kamal applies for SERVICE from this
# checkout: its deploy config, .kamal/secrets and the env of its deploy step.
config_hash() {
  {
    cat "config/deploy.$1.yml" .kamal/secrets 2>/dev/null
    awk -v n="- name: Deploy $1 via Kamal" '
      /^ *- name: / { on = index($0, n) > 0 }
      on && !/^ *#/' .github/workflows/main.yml 2>/dev/null
  } | sha256sum | cut -d' ' -f1
}

# live_config_hash SERVICE: the hash labelled on the live containers, empty when
# they carry none or disagree; fails when the host can't be asked.
live_config_hash() {
  local out hashes
  out=$(ssh "deploy@${KAMAL_SERVER_IP:?}" docker ps \
    --filter "label=service=tools-xdoubleu-com-$1" \
    --format '{{.Labels}}' 2>/dev/null) || return 1
  mapfile -t hashes < <(grep -o 'deploy-config-hash=[0-9a-f]*' <<<"$out" | cut -d= -f2 | sort -u || true)
  [[ ${#hashes[@]} -eq 1 ]] && echo "${hashes[0]}"
  return 0
}

# config_stale SERVICE: whether the live containers lack this checkout's config.
# A job whose checkout differs from main's tip never claims it: the tip's own
# job applies its config.
config_stale() {
  local live
  if ! live=$(live_config_hash "$1"); then
    log "::warning::Could not read the live $1 config hash; not redeploying for config"
    return 1
  fi
  [[ $live != "$(config_hash "$1")" ]] || return 1
  [[ -n $2 ]] && same_deploy_config "$2" || return 1
  log "::notice::$1: the live config differs from this checkout's; redeploying ${live:+(live hash $live)}"
}

# A job of an older commit deploys with its own checkout, so a newer commit that
# changed the deploy config or secrets can't be shipped safely from it.
same_deploy_config() {
  git cat-file -e "$1^{commit}" 2>/dev/null || git fetch --no-tags --depth=1 --quiet origin "$1"
  git diff --quiet "$GITHUB_SHA" "$1" -- config .kamal Gemfile Gemfile.lock
}

main() {
  [[ $# -gt 0 ]] || { log "usage: $0 api|grafana..."; exit 2; }
  : "${GITHUB_REPOSITORY:?}" "${GITHUB_SHA:?}"
  local attempt=${GITHUB_RUN_ATTEMPT:-1} out=${GITHUB_OUTPUT:-/dev/stdout}
  local cands svc deployed target run tip
  cands=$(candidates)
  tip=$(awk 'NR == 1 {print $1}' <<<"$cands")
  for svc in "$@"; do
    [[ $svc == api || $svc == grafana ]] || { log "unknown service: $svc"; exit 2; }
    deployed=$(deployed_version "$svc")
    target=$(decide "$svc" "$deployed" "$GITHUB_SHA" "$attempt" <<<"$cands")
    if [[ $target != skip && $target != "$GITHUB_SHA" ]]; then
      if ! same_deploy_config "$target"; then
        run=$(awk -v s="$target" '$1 == s {print $2}' <<<"$cands")
        log "::error::$svc: main has $target, but config/, .kamal/ or Gemfile* differ from this run's $GITHUB_SHA, so this job can't ship it. Re-run \"Deploy to Hetzner via Kamal\" on ${GITHUB_SERVER_URL:-https://github.com}/$GITHUB_REPOSITORY/actions/runs/$run"
        exit 1
      fi
      log "::notice::$svc: shipping $target, newer than this run's $GITHUB_SHA (live: ${deployed:-unknown})"
    fi
    if [[ $target == skip && -n $deployed ]]; then
      if [[ ${FORCE_DEPLOY:-} == true ]]; then
        log "::notice::$svc: force_deploy redeploys the live $deployed"
        target=$deployed
      elif config_stale "$svc" "$tip"; then
        target=$deployed
      fi
    fi
    echo "$svc=$target" >>"$out"
    echo "${svc}_config_hash=$(config_hash "$svc")" >>"$out"
  done
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
  main "$@"
fi
