#!/usr/bin/env bash
# Usage: fetch_doc_budgets.sh REV DIR. Puts xdoubleu/skills' plugins/doc-budgets
# at REV into DIR. With WARDCASE_PROXY set (a factory job, whose only network is
# the egress proxy) it takes GitHub's tarball of REV through the proxy's
# read-only /nixsrc/ route; otherwise it clones from GitHub.
set -euo pipefail
rev=$1 dir=$2
tmp="$dir.tmp"
rm -rf "$tmp"
mkdir -p "$tmp"
trap 'rm -rf "$tmp"' EXIT
if [[ -n "${WARDCASE_PROXY:-}" ]]; then
  curl -fsS --noproxy '*' "$WARDCASE_PROXY/nixsrc/xdoubleu/skills/tar.gz/$rev" |
    tar -xzf - -C "$tmp" --strip-components=1
else
  git clone -q https://github.com/xdoubleu/skills "$tmp"
  git -C "$tmp" -c advice.detachedHead=false checkout -q "$rev"
fi
[[ -x "$tmp/plugins/doc-budgets/scripts/lint_docs.sh" ]] || {
  echo "fetch_doc_budgets: no plugins/doc-budgets/scripts/lint_docs.sh at $rev" >&2
  exit 1
}
mkdir -p "$(dirname "$dir")"
mv "$tmp/plugins/doc-budgets" "$dir"
