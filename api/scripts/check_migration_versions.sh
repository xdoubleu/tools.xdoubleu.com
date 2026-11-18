#!/usr/bin/env bash
# Fails if two migration files in the same directory share a version number,
# or if a migration added on this branch isn't a 14-digit UTC timestamp
# (YYYYMMDDHHMMSS_name.sql, from `make migration/new`).
#
# goose.Up refuses a duplicate version before applying anything, so the API
# fails at startup. Sequential numbers (000NN) let two parallel branches pick
# the same next number without a git conflict; timestamps never collide.
# Existing 000NN files stay as they are.
set -euo pipefail

# The globs below are relative to api/; elsewhere they match nothing and pass.
cd "$(dirname "$0")/.."

status=0

for dir in cmd/api/migrations apps/*/migrations internal/platform/*/migrations; do
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
	echo "goose refuses duplicate versions and the API won't start — rename the newer file."
fi

# Format check: any migration added since the branch point, committed or
# not, must use a timestamp version. Requires enough git history to resolve
# BASE_REF (default origin/main) — a shallow local clone without that
# remote-tracking ref just skips this part.
base_ref="${BASE_REF:-origin/main}"

if merge_base=$(git merge-base "$base_ref" HEAD 2>/dev/null); then
	bad_format=0

	for dir in cmd/api/migrations apps/*/migrations internal/platform/*/migrations; do
		[ -d "$dir" ] || continue

		# Committed, staged, unstaged and untracked additions all count, so
		# a local run catches what CI would.
		added_files=$(
			{
				git diff --name-only --diff-filter=A "$merge_base" -- "$dir"
				git ls-files --others --exclude-standard -- "$dir"
			} | xargs -r -n1 basename | sort -u
		)

		for file in $added_files; do
			if ! printf '%s\n' "$file" | grep -qE '^[0-9]{14}_[^[:space:]]+\.sql$'; then
				echo "migration $file in $dir is not timestamp-versioned" \
					"(YYYYMMDDHHMMSS_name.sql)"
				bad_format=1
			fi
		done
	done

	if [ "$bad_format" -ne 0 ]; then
		status=1
		echo
		echo "New migrations use a 14-digit UTC timestamp so parallel branches" \
			"never collide — rename it, or create it with:" \
			"make migration/new app=<app> name=<name>"
	fi
else
	echo "warning: could not resolve $base_ref — skipping migration" \
		"format check" >&2
fi

exit "$status"
