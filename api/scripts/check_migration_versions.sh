#!/usr/bin/env bash
# Fails if two migration files in the same directory share a version number.
#
# Goose tracks applied migrations by version number, not filename, so a
# duplicate number is not an error to it — it treats the version as already
# applied and skips the second file silently, recording success while the
# schema change never runs. Two branches each adding e.g. 00022_*.sql pass CI
# independently; the collision only exists once the second one rebases, which
# is exactly when this check fires.
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

exit "$status"
