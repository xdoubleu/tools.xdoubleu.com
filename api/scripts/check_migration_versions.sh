#!/usr/bin/env bash
# Fails if two migration files in the same directory share a version number,
# or if a migration added on this branch isn't numbered above every migration
# that already existed in its directory at the branch point.
#
# Goose tracks applied migrations by version number, not filename, so a
# duplicate number is not an error to it — it treats the version as already
# applied and skips the second file silently, recording success while the
# schema change never runs. Two branches each adding e.g. 00022_*.sql pass CI
# independently; the collision only exists once the second one rebases, which
# is exactly when this check fires. A non-duplicate out-of-order insert (main
# already has 00023_*.sql, this branch adds 00022_*.sql) doesn't collide, but
# it still silently changes goose's run order relative to what the author
# tested against, so it's caught separately below.
set -euo pipefail

status=0

for dir in cmd/api/migrations apps/*/migrations; do
	[ -d "$dir" ] || continue

	duplicates=$(
		find "$dir" -maxdepth 1 -name '*.sql' -exec basename {} \; |
			cut -d_ -f1 |
			sort |
			uniq -d
	)

	for version in $duplicates; do
		echo "duplicate migration version $version in $dir:"
		find "$dir" -maxdepth 1 -name "${version}_*.sql" -exec basename {} \; |
			sort |
			sed 's/^/  /'
		status=1
	done
done

if [ "$status" -ne 0 ]; then
	echo
	echo "Goose skips a duplicate version silently — renumber the newer file."
fi

# Out-of-order check: any migration added since the branch point must have a
# higher version than every migration that already existed in its directory.
# Requires enough git history to resolve BASE_REF (default origin/main) — a
# shallow local clone without that remote-tracking ref just skips this part.
base_ref="${BASE_REF:-origin/main}"
repo_prefix=$(git rev-parse --show-prefix)

if merge_base=$(git merge-base "$base_ref" HEAD 2>/dev/null); then
	for dir in cmd/api/migrations apps/*/migrations; do
		[ -d "$dir" ] || continue

		base_max=$(
			git show "$merge_base:${repo_prefix}${dir}" 2>/dev/null |
				grep -o '^[0-9]\{1,\}_[^[:space:]]*\.sql$' |
				cut -d_ -f1 |
				sort -n |
				tail -1
		)
		[ -n "$base_max" ] || base_max=0

		added_files=$(
			git diff --name-only --diff-filter=A "$merge_base" HEAD -- "${repo_prefix}${dir}" |
				xargs -r -n1 basename
		)

		for file in $added_files; do
			version=$(echo "$file" | cut -d_ -f1)
			# base 10 avoids octal misparse of zero-padded versions like 00008
			if [ "$((10#$version))" -le "$((10#$base_max))" ]; then
				echo "migration $file in $dir is numbered $version," \
					"which is not above the existing max $base_max"
				status=1
			fi
		done
	done

	if [ "$status" -ne 0 ]; then
		echo
		echo "A new migration must be numbered higher than every migration" \
			"already in its directory — renumber it."
	fi
else
	echo "warning: could not resolve $base_ref — skipping out-of-order" \
		"migration check" >&2
fi

exit "$status"
