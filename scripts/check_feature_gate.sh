#!/usr/bin/env bash
# Fails a `feature`-labelled PR whose body lacks the feature-review gate's
# evidence: a line `Feature gate: mutation=pass, review=pass`. Anything else,
# including `mutation=skipped`, blocks the merge.
# Inputs (env): PR_LABELS (comma- or newline-separated label names), PR_BODY.
set -uo pipefail

if ! tr "," "\n" <<<"${PR_LABELS:-}" | grep -qxi feature; then
  echo "Not a feature PR: no gate evidence needed."
  exit 0
fi

line=$(grep -im1 '^[[:space:]]*Feature gate:' <<<"${PR_BODY:-}" || true)
if [[ -z "$line" ]]; then
  echo "::error::feature PR has no 'Feature gate:' line in its body (docs/convention-feature-review-policy.md)"
  exit 1
fi

for want in mutation review; do
  if ! grep -qiE "(^|[ ,:])${want}=pass([ ,;.]|$)" <<<"$line"; then
    echo "::error::'$line' lacks ${want}=pass; run the gate or leave the PR blocked"
    exit 1
  fi
done
echo "Feature gate evidence present: $line"
