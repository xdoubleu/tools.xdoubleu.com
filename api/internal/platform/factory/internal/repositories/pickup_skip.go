package repositories

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/kit/database/postgres"
)

// skipPrefix marks a pickup_blocked reason NextReady set itself, so it can
// re-check and clear it, unlike a repo-settings block that needs a fresh Ready move.
const skipPrefix = "Skipped by auto pickup: "

var (
	// dependsOn finds "Depends on" at the start of a line, or "Depends on:" (colon
	// required) at the start of a later sentence on it, so "Part of X (T-1).
	// Depends on: T-2." counts and "the slice that depends on T-3" does not.
	dependsOn = regexp.MustCompile(
		`(?im)^\W*depends on\b:?(.*)$|[.;!?]\s+\W*depends on\s*:(.*)$`)
	taskRef  = regexp.MustCompile(`\bT-(\d+)\b`)
	issueRef = regexp.MustCompile(`#(\d+)`)
	issueKey = regexp.MustCompile(`\[github-issue:(\d+)\]`)
)

// pickupRefs reads what a body says about other work: the T-numbers and GitHub
// issue numbers in its "Depends on" sentences, and its own imported issue key
// ("" if none).
func pickupRefs(body string) ([]int64, []int64, string) {
	var tasks, issues []int64
	for _, m := range dependsOn.FindAllStringSubmatch(body, -1) {
		deps := sentence(m[1] + m[2])
		tasks = append(tasks, numbers(taskRef, deps)...)
		issues = append(issues, numbers(issueRef, deps)...)
	}
	key := ""
	if m := issueKey.FindStringSubmatch(body); m != nil {
		key = m[1]
	}
	return tasks, issues, key
}

// sentence cuts s at the first full stop outside parentheses that ends a sentence,
// so a later sentence on the same line ("T-9 waits on this task") is not a dependency.
func sentence(s string) string {
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '.', ';':
			next := i + 1
			if depth == 0 && (next == len(s) || s[next] == ' ' || s[next] == '\t') {
				return s[:i]
			}
		}
	}
	return s
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

// pickupRow is what pickupSkipReasons needs of a Ready task.
type pickupRow struct {
	id     uuid.UUID
	number int64
	labels []string
	body   string
}

// pickupSkipReasons says, per task that auto pickup must pass by, why: it is an
// epic, it is the parent of another task (whose body says "Part of ... (T-n)"), a
// task or imported GitHub issue it depends on isn't merged, or another task with
// the same imported issue key already has an open job or pull request. It runs a
// constant number of queries however many tasks it vets.
func (r *TasksRepository) pickupSkipReasons(
	ctx context.Context,
	rows []pickupRow,
) (map[uuid.UUID]string, error) {
	reasons := map[uuid.UUID]string{}
	var ids []uuid.UUID
	var numbers []int64
	for _, x := range rows {
		if slices.Contains(x.labels, "epic") {
			reasons[x.id] = "it is an epic; its sub-tasks are picked up instead."
			continue
		}
		ids = append(ids, x.id)
		numbers = append(numbers, x.number)
	}
	if len(ids) == 0 {
		return reasons, nil
	}

	children, err := r.firstChildren(ctx, ids, numbers)
	if err != nil {
		return nil, err
	}
	refs := map[uuid.UUID]pickupDeps{}
	var allTasks, allIssues []int64
	var keyIDs []uuid.UUID
	var keys []string
	for _, x := range rows {
		if _, done := reasons[x.id]; done {
			continue
		}
		if child, ok := children[x.id]; ok {
			reasons[x.id] = fmt.Sprintf(
				"it is the parent of T-%d; pick up the sub-tasks.", child)
			continue
		}
		var d pickupDeps
		d.tasks, d.issues, d.key = pickupRefs(x.body)
		refs[x.id] = d
		allTasks = append(allTasks, d.tasks...)
		allIssues = append(allIssues, d.issues...)
		if d.key != "" {
			keyIDs = append(keyIDs, x.id)
			keys = append(keys, d.key)
		}
	}

	openTasks, openIssues, err := r.openDependencies(ctx, allTasks, allIssues)
	if err != nil {
		return nil, err
	}
	dups, err := r.openDuplicates(ctx, keyIDs, keys)
	if err != nil {
		return nil, err
	}
	for id, d := range refs {
		if reason := dependencyReason(d, openTasks, openIssues, dups[id]); reason != "" {
			reasons[id] = reason
		}
	}
	return reasons, nil
}

// dependencyReason is the skip reason for a task's dependencies, or "": one of them
// isn't merged, or dup (when not 0) holds the same issue with open work.
func dependencyReason(
	d pickupDeps,
	openTasks, openIssues map[int64]bool,
	dup int64,
) string {
	var open []string
	for _, n := range d.tasks {
		if openTasks[n] {
			open = append(open, fmt.Sprintf("T-%d", n))
		}
	}
	for _, k := range d.issues {
		if openIssues[k] {
			open = append(open, fmt.Sprintf("GitHub issue #%d", k))
		}
	}
	if len(open) > 0 {
		slices.Sort(open)
		open = slices.Compact(open)
		return "it depends on " + strings.Join(open, ", ") + ", not merged yet."
	}
	if dup != 0 {
		return fmt.Sprintf(
			"T-%d already has an open job or pull request for GitHub issue #%s.", dup, d.key)
	}
	return ""
}

// pickupDeps is what a task body says about other work.
type pickupDeps struct {
	tasks, issues []int64
	key           string
}

// firstChildren maps each task to the number of one task whose body says it is
// "Part of ... (T-n)" of it.
func (r *TasksRepository) firstChildren(
	ctx context.Context,
	ids []uuid.UUID,
	numbers []int64,
) (map[uuid.UUID]int64, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id, min(c.number)
		FROM unnest($1::uuid[], $2::bigint[]) AS p(id, number)
		JOIN factory.tasks c ON c.id <> p.id
			AND c.body ~ ('Part of [^\n]*\(T-' || p.number::text || '\)')
		GROUP BY p.id`, ids, numbers)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()
	out := map[uuid.UUID]int64{}
	for rows.Next() {
		var id uuid.UUID
		var n int64
		if err = rows.Scan(&id, &n); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out[id] = n
	}
	return out, postgres.PgxErrorToHTTPError(rows.Err())
}

// openDependencies returns which of the given task numbers and imported GitHub
// issue numbers aren't merged yet.
func (r *TasksRepository) openDependencies(
	ctx context.Context,
	depTasks, depIssues []int64,
) (map[int64]bool, map[int64]bool, error) {
	openTasks, openIssues := map[int64]bool{}, map[int64]bool{}
	if len(depTasks) == 0 && len(depIssues) == 0 {
		return openTasks, openIssues, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT 't', number FROM factory.tasks
		WHERE number = ANY($1) AND status NOT IN ('done', 'testing') AND merge_sha = ''
		UNION
		SELECT 'i', k FROM unnest($2::bigint[]) k
		WHERE EXISTS (
			SELECT 1 FROM factory.tasks
			WHERE body ~ ('\[github-issue:' || k || '\]')
		) AND NOT EXISTS (
			SELECT 1 FROM factory.tasks
			WHERE body ~ ('\[github-issue:' || k || '\]')
				AND (status IN ('done', 'testing') OR merge_sha <> '')
		)`, depTasks, depIssues)
	if err != nil {
		return nil, nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var n int64
		if err = rows.Scan(&kind, &n); err != nil {
			return nil, nil, postgres.PgxErrorToHTTPError(err)
		}
		if kind == "t" {
			openTasks[n] = true
		} else {
			openIssues[n] = true
		}
	}
	return openTasks, openIssues, postgres.PgxErrorToHTTPError(rows.Err())
}

// openDuplicates maps each task to the number of another task with the same
// imported issue key that has an open job or pull request.
func (r *TasksRepository) openDuplicates(
	ctx context.Context,
	ids []uuid.UUID,
	keys []string,
) (map[uuid.UUID]int64, error) {
	out := map[uuid.UUID]int64{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT k.id, min(t2.number)
		FROM unnest($1::uuid[], $2::text[]) AS k(id, key)
		JOIN factory.tasks t2 ON t2.id <> k.id
			AND t2.body ~ ('\[github-issue:' || k.key || '\]')
			AND (
				EXISTS (SELECT 1 FROM factory.jobs j WHERE j.task_id = t2.id
					AND (j.status IN ('queued', 'running') OR j.pr_state = 'open'))
				OR EXISTS (SELECT 1 FROM factory.linked_pull_requests l
					WHERE l.task_id = t2.id AND l.state = 'open')
			)
		GROUP BY k.id`, ids, keys)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var n int64
		if err = rows.Scan(&id, &n); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out[id] = n
	}
	return out, postgres.PgxErrorToHTTPError(rows.Err())
}
