package repositories

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
)

// skipPrefix marks a pickup_blocked reason NextReady set itself, so it can
// re-check and clear it, unlike a repo-settings block that needs a fresh Ready move.
const skipPrefix = "Skipped by auto pickup: "

var (
	dependsLine = regexp.MustCompile(`(?im)^\W*depends on\b:?(.*)$`)
	taskRef     = regexp.MustCompile(`\bT-(\d+)\b`)
	issueRef    = regexp.MustCompile(`#(\d+)`)
	issueKey    = regexp.MustCompile(`\[github-issue:(\d+)\]`)
)

// pickupRefs reads what a body says about other work: the T-numbers and GitHub
// issue numbers on its "Depends on" lines, and its own imported issue key ("" if none).
func pickupRefs(body string) ([]int64, []int64, string) {
	var tasks, issues []int64
	for _, line := range dependsLine.FindAllStringSubmatch(body, -1) {
		tasks = append(tasks, numbers(taskRef, line[1])...)
		issues = append(issues, numbers(issueRef, line[1])...)
	}
	key := ""
	if m := issueKey.FindStringSubmatch(body); m != nil {
		key = m[1]
	}
	return tasks, issues, key
}

func numbers(re *regexp.Regexp, s string) []int64 {
	var out []int64
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		if n, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// pickupSkipReason says why auto pickup must pass a Ready task by, or "": it is an
// epic, it is the parent of another task (whose body says "Part of ... (T-n)"), a
// task or imported GitHub issue it depends on isn't merged, or another task with
// the same imported issue key already has an open job or pull request.
func (r *TasksRepository) pickupSkipReason(
	ctx context.Context,
	id uuid.UUID,
	number int64,
	labels []string,
	body string,
) (string, error) {
	if slices.Contains(labels, "epic") {
		return "it is an epic; its sub-tasks are picked up instead.", nil
	}
	depTasks, depIssues, key := pickupRefs(body)

	var child *int64
	err := r.db.QueryRow(ctx, `
		SELECT number FROM factory.tasks
		WHERE id <> $1 AND body ~ ('Part of [^\n]*\(T-' || $2::bigint::text || '\)')
		LIMIT 1`, id, number).Scan(&child)
	if err == nil {
		return fmt.Sprintf("it is the parent of T-%d; pick up the sub-tasks.", *child), nil
	}
	if e := postgres.PgxErrorToHTTPError(err); !isMissing(e) {
		return "", e
	}

	var open []string
	rows, err := r.db.Query(ctx, `
		SELECT 'T-' || number FROM factory.tasks
		WHERE number = ANY($1) AND status NOT IN ('done', 'testing') AND merge_sha = ''
		UNION
		SELECT 'GitHub issue #' || k FROM unnest($2::bigint[]) k
		WHERE EXISTS (
			SELECT 1 FROM factory.tasks
			WHERE body ~ ('\[github-issue:' || k || '\]')
		) AND NOT EXISTS (
			SELECT 1 FROM factory.tasks
			WHERE body ~ ('\[github-issue:' || k || '\]')
				AND (status IN ('done', 'testing') OR merge_sha <> '')
		)`, depTasks, depIssues)
	if err != nil {
		return "", postgres.PgxErrorToHTTPError(err)
	}
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			rows.Close()
			return "", postgres.PgxErrorToHTTPError(err)
		}
		open = append(open, s)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return "", postgres.PgxErrorToHTTPError(err)
	}
	if len(open) > 0 {
		slices.Sort(open)
		return "it depends on " + strings.Join(open, ", ") + ", not merged yet.", nil
	}

	if key == "" {
		return "", nil
	}
	var dup int64
	err = r.db.QueryRow(ctx, `
		SELECT t2.number FROM factory.tasks t2
		WHERE t2.id <> $1 AND t2.body ~ ('\[github-issue:' || $2::text || '\]')
			AND (
				EXISTS (SELECT 1 FROM factory.jobs j WHERE j.task_id = t2.id
					AND (j.status IN ('queued', 'running') OR j.pr_state = 'open'))
				OR EXISTS (SELECT 1 FROM factory.linked_pull_requests l
					WHERE l.task_id = t2.id AND l.state = 'open')
			)
		LIMIT 1`, id, key).Scan(&dup)
	if err == nil {
		return fmt.Sprintf(
			"T-%d already has an open job or pull request for GitHub issue #%s.", dup, key), nil
	}
	if e := postgres.PgxErrorToHTTPError(err); !isMissing(e) {
		return "", e
	}
	return "", nil
}

func isMissing(err error) bool {
	return errors.Is(err, database.ErrResourceNotFound)
}
