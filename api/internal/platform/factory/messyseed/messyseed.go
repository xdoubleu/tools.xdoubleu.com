// Package messyseed fills the factory schema with production-shaped data for
// the read tests (`make test/reads`).
package messyseed

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/database/postgres"
)

// Board is what production data looks like, as opposed to what a feature
// test creates: every link table fans out (one pull request on several tasks,
// one task with several pull requests and jobs), jobs end in every way, and
// text is empty or Unicode. Add a row here for every new relation, so
// every TestMessyReads* test sees it.
type Board struct {
	TaskIDs []string
	JobIDs  []string
}

const messyPR = "https://github.com/xdoubleu/tools.xdoubleu.com/pull/"

type seeder struct {
	t  testing.TB
	db postgres.DB
}

func (s seeder) exec(sql string, args ...any) {
	s.t.Helper()
	_, err := s.db.Exec(context.Background(), sql, args...)
	require.NoError(s.t, err, sql)
}

func (s seeder) ids(sql string, args ...any) []string {
	s.t.Helper()
	rows, err := s.db.Query(context.Background(), sql, args...)
	require.NoError(s.t, err, sql)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		require.NoError(s.t, rows.Scan(&id))
		out = append(out, id)
	}
	require.NoError(s.t, rows.Err())
	return out
}

func (s seeder) taskID(title string) string {
	s.t.Helper()
	got := s.ids(`SELECT id::text FROM factory.tasks WHERE title = $1`, title)
	require.Len(s.t, got, 1, title)
	return got[0]
}

// Seed replaces the board with the messy one.
func Seed(t testing.TB, db postgres.DB) Board {
	t.Helper()
	s := seeder{t: t, db: db}
	s.reset()
	s.tasks()
	s.links()
	s.jobs()
	return Board{
		TaskIDs: s.ids(`SELECT id::text FROM factory.tasks ORDER BY number`),
		JobIDs:  s.ids(`SELECT id::text FROM factory.jobs ORDER BY created_at, id`),
	}
}

func (s seeder) reset() {
	s.exec(`DELETE FROM factory.tasks`)
	s.exec(`DELETE FROM factory.jobs`)
	s.exec(`DELETE FROM factory.routines WHERE name <> 'noop'`)
	s.exec(`DELETE FROM factory.repos WHERE name = 'messy-second'`)
	s.exec(`INSERT INTO factory.repos (owner, name) VALUES ('xdoubleu', 'messy-second')`)
}

// tasks puts one task in every column, with odd text.
func (s seeder) tasks() {
	// One task per column, with odd text.
	s.exec(`
		INSERT INTO factory.tasks (
			title, body, status, priority, labels,
			created_by, body_edited_by, body_edited_channel,
			repo_id, merge_sha, pickup_blocked, ready_title, ready_body
		)
		SELECT
			s.title, s.body, s.status, s.priority, s.labels,
			'user:a', 'user:a', 'browser', r.id,
			CASE WHEN s.status = 'done' THEN 'abc123' ELSE '' END,
			CASE WHEN s.status = 'ready' THEN 'T-1 first' ELSE '' END,
			CASE WHEN s.status = 'ready' THEN s.title END,
			CASE WHEN s.status = 'ready' THEN s.body END
		FROM (VALUES
			('backlog 🚀', '', 'backlog', 'p2', '{}'::text[]),
			('', 'empty title', 'ready', 'p0', '{bug,"ünïcode ✓"}'),
			('ready two', 'multi
line "quoted" ''body''', 'ready', 'p1', '{feature}'),
			('in progress', 'x', 'in_progress', 'p2', '{}'),
			('in review', 'x', 'in_review', 'p2', '{}'),
			('needs you', 'x', 'needs_you', 'p1', '{}'),
			('testing', 'x', 'testing', 'p2', '{}'),
			('done', 'x', 'done', 'p2', '{}'),
			('parked', 'x', 'in_progress', 'p2', '{waiting-on-sandbox}'),
			('shared pr a', 'x', 'in_review', 'p2', '{}'),
			('shared pr b', 'x', 'in_review', 'p2', '{}'),
			('busy', 'x', 'in_review', 'p2', '{}')
		) AS s(title, body, status, priority, labels)
		CROSS JOIN factory.repos r
		WHERE r.name = 'tools.xdoubleu.com'`)
}

// links fans comments and pull requests out over the tasks.
func (s seeder) links() {
	var (
		sharedA = s.taskID("shared pr a")
		sharedB = s.taskID("shared pr b")
		busy    = s.taskID("busy")
	)

	s.exec(`
		INSERT INTO factory.task_comments (task_id, author, body)
		SELECT id, 'user:a', '' FROM factory.tasks
		UNION ALL SELECT id, 'agent:b', 'çomment 💬' FROM factory.tasks
		UNION ALL SELECT id, 'agent:b', 'second' FROM factory.tasks`)

	// One pull request linked to several tasks, the job's PR the same one.
	s.exec(`
		INSERT INTO factory.linked_pull_requests
			(url, task_id, state, ci, review, mergeable)
		VALUES ($1, $3, 'open', 'passing', 'approved', 'clean'),
			($1, $4, 'open', 'passing', 'approved', 'clean'),
			($2, $4, 'merged', 'passing', '', ''),
			($5, $6, 'closed', 'failing', 'changes_requested', 'dirty'),
			($7, $6, 'open', 'pending', '', '')`,
		messyPR+"1", messyPR+"2", sharedA, sharedB, messyPR+"3", busy, messyPR+"4")
}

// jobs ends jobs in every way, with events, and adds routines.
func (s seeder) jobs() {
	var (
		sharedA = s.taskID("shared pr a")
		sharedB = s.taskID("shared pr b")
		busy    = s.taskID("busy")
		parked  = s.taskID("parked")
	)

	job := func(task, status, pr, state, session, routine string, parent *string) {
		s.t.Helper()
		s.exec(`
			INSERT INTO factory.jobs (
				task_id, repo_id, harness, prompt, status, branch, session_id, outcome,
				created_by, pr_url, pr_state, pr_ci, pr_review, parent_job_id, routine,
				started_at, finished_at, limited, limit_resets_at
			)
			SELECT NULLIF($1, '')::uuid, r.id, 'claude_code', 'do 🚀', $2, 'agent/x', $5,
				'', 'user:a', $3, $4, 'passing', 'approved', $7::uuid, $6,
				CASE WHEN $2 <> 'queued' THEN now() END,
				CASE WHEN $2 IN ('succeeded', 'failed', 'cancelled', 'lost')
					THEN now() END,
				$2 = 'failed', CASE WHEN $2 = 'failed' THEN now() END
			FROM factory.repos r WHERE r.name = 'tools.xdoubleu.com'`,
			task, status, pr, state, session, routine, parent)
	}
	job(sharedA, "succeeded", messyPR+"1", "open", "s1", "", nil)
	job(sharedB, "succeeded", messyPR+"1", "open", "", "", nil)
	job(sharedB, "succeeded", messyPR+"2", "merged", "s2", "", nil)
	// Retried, lost and failed jobs on one task.
	job(busy, "failed", messyPR+"3", "closed", "s3", "", nil)
	job(busy, "lost", "", "", "", "", nil)
	job(busy, "cancelled", "", "", "", "", nil)
	first := s.ids(`SELECT id::text FROM factory.jobs WHERE task_id = $1
		ORDER BY created_at LIMIT 1`, busy)[0]
	job(busy, "succeeded", messyPR+"4", "open", "s4", "", &first)
	job(parked, "succeeded", "", "", "s5", "", nil)
	job(s.taskID("in progress"), "running", "", "", "s6", "", nil)
	job(s.taskID("ready two"), "queued", "", "", "", "", nil)
	// Routine jobs have no task.
	job("", "succeeded", "", "", "", "noop", nil)
	job("", "running", "", "", "", "noop", nil)

	s.exec(`
		INSERT INTO factory.job_events (job_id, seq, kind, payload)
		SELECT id, s, 'text', CASE WHEN s = 1 THEN '' ELSE '日本語 ✓' END
		FROM factory.jobs, generate_series(1, 3) AS s`)
	s.exec(`INSERT INTO factory.routines (name, repo_id, enabled, interval_minutes)
		SELECT 'messy-routine', id, true, 60 FROM factory.repos
		WHERE name = 'messy-second'`)
}
