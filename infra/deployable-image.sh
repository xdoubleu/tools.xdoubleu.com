#!/bin/sh
# Newest ancestor of HEAD whose merged `app` image was actually pushed to GHCR,
# as JSON for main.tf's data "external" "deployable_image".
#
# HEAD itself often has no image: docker.yml only builds and pushes the merged
# `app` image when an app subtree changed, so any infra-only commit (this
# directory, docs) leaves the previous commit's tag as the newest one that
# exists. Deriving Kamal's --version from `git rev-parse HEAD` regardless is
# what produced `failed to resolve reference ghcr.io/.../app:6ce301d: not
# found` on the first deploy after an infra-only commit (issue #1036).
#
# Probes manifests one commit at a time rather than reading /tags/list: that
# endpoint is paginated (100 per page) and not ordered newest-first, so
# picking "the newest tag" from it would need to page through the whole
# history. Walking git history instead answers the question directly, and
# normally hits on the first or second try.
#
# ponytail: anonymous pull token — the package is public, so no credentials
# are involved here. Needs a registry login step if it ever goes private.
set -eu

REPO=xdoubleu/tools.xdoubleu.com/app
REPO_DIR=$(dirname "$0")

TOKEN=$(curl -sf "https://ghcr.io/token?scope=repository:$REPO:pull&service=ghcr.io" |
  sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
[ -n "$TOKEN" ] || { echo "could not get an anonymous ghcr.io pull token" >&2; exit 1; }

for sha in $(git -C "$REPO_DIR" rev-list -n 50 HEAD); do
  tag=$(echo "$sha" | cut -c1-7)
  if curl -sf -o /dev/null \
    -H "Authorization: Bearer $TOKEN" \
    -H "Accept: application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.list.v2+json,application/vnd.docker.distribution.manifest.v2+json" \
    "https://ghcr.io/v2/$REPO/manifests/$tag"; then
    printf '{"tag":"%s","sha":"%s"}' "$tag" "$sha"
    exit 0
  fi
done

echo "no ghcr.io/$REPO image found for HEAD or its 49 closest ancestors" >&2
exit 1
