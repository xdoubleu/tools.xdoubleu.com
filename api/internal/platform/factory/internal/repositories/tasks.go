package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// prFields is how many values prColumn holds.
// pickupAuthor attributes the comments auto pickup leaves on a task it passes over.
const pickupAuthor = "system:auto-pickup"

const prFields = 5

type TasksRepository struct {
	db postgres.DB
}

type rowScanner interface {
	Scan(dest ...any) error
}

// prColumn is the task's pull request that is still open, else its newest, among
// those its jobs opened and those linking it in their body, as
// {url, state, ci, review, mergeable}; null when it has none.
const prColumn = `(SELECT ARRAY[u.url, u.state, u.ci, u.review, u.mergeable] FROM (
		SELECT j.pr_url AS url, j.pr_state AS state, j.pr_ci AS ci,
			j.pr_review AS review, j.created_at AS ts,
			COALESCE((SELECT m.mergeable FROM factory.linked_pull_requests m
				WHERE m.url = j.pr_url LIMIT 1), '') AS mergeable
		FROM factory.jobs j WHERE j.task_id = t.id AND j.pr_url <> ''
		UNION ALL
		SELECT l.url, l.state, l.ci, l.review, l.checked_at, l.mergeable
		FROM factory.linked_pull_requests l WHERE l.task_id = t.id) u
		ORDER BY (u.state = 'open') DESC, u.ts DESC, u.url LIMIT 1)`

// listColumns leaves out body: list rows never carry the wide column.
const listColumns = `id, number, repo_id, title, '', status, priority, labels,
	created_by, created_at, updated_at,
	(SELECT count(*) FROM factory.task_comments c WHERE c.task_id = t.id),
	` + prColumn + `,
	ready_body IS NOT NULL, pickup_blocked, merge_sha`

const getColumns = `id, number, repo_id, title, body, status, priority, labels,
	created_by, created_at, updated_at,
	(SELECT count(*) FROM factory.task_comments c WHERE c.task_id = t.id),
	` + prColumn + `,
	ready_body IS NOT NULL, pickup_blocked, merge_sha`

func scanTask(row rowScanner) (*models.Task, error) {
	var t models.Task
	var pr []string
	err := row.Scan(
		&t.ID, &t.Number, &t.RepoID, &t.Title, &t.Body, &t.Status, &t.Priority, &t.Labels,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CommentCount, &pr,
		&t.PickupApproved, &t.PickupBlocked, &t.MergeSHA,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	if len(pr) == prFields {
		t.PRURL, t.PRState, t.PRCI, t.PRReview = pr[0], pr[1], pr[2], pr[3]
		t.PRMergeable = pr[4]
	}
	return &t, nil
}

// List returns tasks without their bodies, newest first. A non-empty query keeps
// only tasks whose title or body contains it, ignoring case: the match runs in
// SQL so bodies needn't be loaded.
func (r *TasksRepository) List(
	ctx context.Context,
	query string,
) ([]models.Task, error) {
	return r.Find(ctx, models.TaskFilter{
		Query: query, Status: "", RepoID: nil, Label: "",
	})
}

// Find is List narrowed by every non-empty field of the filter.
func (r *TasksRepository) Find(
	ctx context.Context,
	f models.TaskFilter,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+listColumns+`
		FROM factory.tasks t
		WHERE ($1 = '' OR position(lower($1) in lower(t.title || E'\n' || t.body)) > 0)
			AND ($2::text = '' OR t.status::text = $2::text)
			AND ($3::uuid IS NULL OR t.repo_id = $3::uuid)
			AND ($4 = '' OR EXISTS (
				SELECT 1 FROM unnest(t.labels) l WHERE lower(l) = lower($4)))
		ORDER BY updated_at DESC, id`,
		f.Query, string(f.Status), f.RepoID, f.Label)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Task{}
	for rows.Next() {
		t, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// Statuses returns the status of each of the given tasks that exists.
func (r *TasksRepository) Statuses(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]models.Status, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, status FROM factory.tasks WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID]models.Status, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var status models.Status
		if err = rows.Scan(&id, &status); err != nil {
			return nil, err
		}
		out[id] = status
	}
	return out, rows.Err()
}

func (r *TasksRepository) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	return scanTask(r.db.QueryRow(ctx, `
		SELECT `+getColumns+` FROM factory.tasks t WHERE id = $1`, id))
}

// ByKeys loads the tasks with the given keys or numbers in one query.
func (r *TasksRepository) ByKeys(
	ctx context.Context,
	ids []uuid.UUID,
	numbers []int64,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+getColumns+` FROM factory.tasks t
		WHERE id = ANY($1) OR number = ANY($2)`, ids, numbers)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	out := []models.Task{}
	for rows.Next() {
		t, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *t)
	}
	return out, postgres.PgxErrorToHTTPError(rows.Err())
}

// Resolve turns a task reference ("T-142", "142" or a legacy UUID) into its key.
func (r *TasksRepository) Resolve(ctx context.Context, ref string) (uuid.UUID, error) {
	if id, err := uuid.Parse(ref); err == nil {
		return id, nil
	}
	number, ok := models.ParseTaskRef(ref)
	if !ok {
		return uuid.Nil, models.ErrInvalidTaskRef
	}
	return r.IDByNumber(ctx, number)
}

// IDByNumber is the key of the task with the given human-readable number.
func (r *TasksRepository) IDByNumber(
	ctx context.Context,
	number int64,
) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT id FROM factory.tasks WHERE number = $1`, number).Scan(&id)
	if err != nil {
		return uuid.Nil, postgres.PgxErrorToHTTPError(err)
	}
	return id, nil
}

// Create inserts a task in Backlog.
func (r *TasksRepository) Create(
	ctx context.Context,
	t models.Task,
	actor models.Actor,
) (*models.Task, error) {
	return scanTask(r.db.QueryRow(ctx, `
		WITH t AS (
			INSERT INTO factory.tasks
				(repo_id, title, body, priority, labels, created_by,
				 body_edited_by, body_edited_channel)
			VALUES ($1, $2, $3, $4, $5, $6, $6, $7)
			RETURNING *
		)
		SELECT `+getColumns+` FROM t`,
		t.RepoID, t.Title, t.Body, t.Priority, t.Labels, actor.String(),
		actor.Channel(),
	))
}

// Update replaces title, body, priority and labels. The body's editor is
// recorded only when the body changes.
func (r *TasksRepository) Update(
	ctx context.Context,
	t models.Task,
	actor models.Actor,
) (*models.Task, error) {
	return scanTask(r.db.QueryRow(ctx, `
		WITH t AS (
			UPDATE factory.tasks SET
				title = $2,
				priority = $4,
				labels = $5,
				body_edited_by = CASE WHEN body IS DISTINCT FROM $3
					THEN $6 ELSE body_edited_by END,
				body_edited_channel = CASE WHEN body IS DISTINCT FROM $3
					THEN $7 ELSE body_edited_channel END,
				ready_title = CASE WHEN status = 'ready' AND (
					CASE WHEN body IS DISTINCT FROM $3 THEN $7
						ELSE body_edited_channel END
				) = 'browser' THEN $2 ELSE ready_title END,
				ready_body = CASE WHEN status = 'ready' AND (
					CASE WHEN body IS DISTINCT FROM $3 THEN $7
						ELSE body_edited_channel END
				) = 'browser' THEN $3 ELSE ready_body END,
				body = $3,
				updated_at = now()
			WHERE id = $1
			RETURNING *
		)
		SELECT `+getColumns+` FROM t`,
		t.ID, t.Title, t.Body, t.Priority, t.Labels, actor.String(),
		actor.Channel(),
	))
}

func (r *TasksRepository) Move(
	ctx context.Context,
	id uuid.UUID,
	status models.Status,
) (*models.Task, error) {
	return scanTask(r.db.QueryRow(ctx, `
		WITH t AS (
			UPDATE factory.tasks SET status = $2, updated_at = now(),
				ready_title = CASE WHEN $2 = 'ready' THEN title END,
				ready_body = CASE WHEN $2 = 'ready' THEN body END,
				-- Retry pickup is a fresh Ready move: clear the block reason when the
				-- task is opened for work again or returned to Backlog.
				pickup_blocked = CASE WHEN $2 IN ('ready', 'backlog')
					THEN '' ELSE pickup_blocked END,
				-- Any move ends a wait; backlogWaiting records it after its move.
				waiting_on = '{}'
			WHERE id = $1
			RETURNING *
		)
		SELECT `+getColumns+` FROM t`, id, status))
}

// SetWaitingOn records the task numbers a Backlog task waits on.
func (r *TasksRepository) SetWaitingOn(
	ctx context.Context,
	id uuid.UUID,
	numbers []int64,
) error {
	_, err := r.db.Exec(ctx,
		`UPDATE factory.tasks SET waiting_on = $2 WHERE id = $1`, id, numbers)
	return postgres.PgxErrorToHTTPError(err)
}

// WaitOver lists the Backlog tasks whose waited-on tasks are all Done, in
// Testing or merged. A number with no task counts as done.
func (r *TasksRepository) WaitOver(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id FROM factory.tasks t
		WHERE t.status = 'backlog' AND cardinality(t.waiting_on) > 0
			AND NOT EXISTS (
				SELECT 1 FROM factory.tasks d
				WHERE d.number = ANY(t.waiting_on)
					AND d.status NOT IN ('done', 'testing') AND d.merge_sha = ''
			)
		ORDER BY t.number`)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	return ids, postgres.PgxErrorToHTTPError(err)
}

// SetLabels replaces a task's labels without touching anything else.
func (r *TasksRepository) SetLabels(
	ctx context.Context,
	id uuid.UUID,
	labels []string,
) error {
	_, err := r.db.Exec(ctx,
		`UPDATE factory.tasks SET labels = $2, updated_at = now() WHERE id = $1`,
		id, labels)
	return err
}

// SetMergeSHA records the commit a task's pull request merged as.
func (r *TasksRepository) SetMergeSHA(
	ctx context.Context,
	id uuid.UUID,
	sha string,
) error {
	_, err := r.db.Exec(ctx,
		`UPDATE factory.tasks SET merge_sha = $2 WHERE id = $1`, id, sha)
	return err
}

// BlockPickup records why a Ready task can't be auto-picked up, so NextReady
// passes it by. It never changes the status: the task stays where it is.
func (r *TasksRepository) BlockPickup(
	ctx context.Context,
	id uuid.UUID,
	reason string,
) (*models.Task, error) {
	return scanTask(r.db.QueryRow(ctx, `
		WITH t AS (
			UPDATE factory.tasks SET pickup_blocked = $2, updated_at = now()
			WHERE id = $1
			RETURNING *
		)
		SELECT `+getColumns+` FROM t`, id, reason))
}

// AddComment appends a comment and bumps the task's updated_at.
func (r *TasksRepository) AddComment(
	ctx context.Context,
	taskID uuid.UUID,
	author string,
	body string,
) (*models.Comment, error) {
	var c models.Comment
	err := r.db.QueryRow(ctx, `
		WITH touched AS (
			UPDATE factory.tasks SET updated_at = now() WHERE id = $1
			RETURNING id
		)
		INSERT INTO factory.task_comments (task_id, author, body)
		SELECT id, $2, $3 FROM touched
		RETURNING id, task_id, author, body, created_at`,
		taskID, author, body,
	).Scan(&c.ID, &c.TaskID, &c.Author, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &c, nil
}

func (r *TasksRepository) Comments(
	ctx context.Context,
	taskID uuid.UUID,
) ([]models.Comment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, author, body, created_at
		FROM factory.task_comments
		WHERE task_id = $1
		ORDER BY created_at, id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Comment{}
	for rows.Next() {
		var c models.Comment
		if err = rows.Scan(
			&c.ID, &c.TaskID, &c.Author, &c.Body, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// repoBlock is the Ready task of a repo with auto pickup off.
type repoBlock struct {
	id   uuid.UUID
	repo string
}

// NoteAutoPickupOff comments once on each approved Ready task whose repo has auto
// pickup off, so it never waits silently. The skip prefix lets NextReady clear
// the block, and pick the task up, once auto pickup is on.
func (r *TasksRepository) NoteAutoPickupOff(ctx context.Context) error {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, r.owner || '/' || r.name
		FROM factory.tasks t
		JOIN factory.repos r ON r.id = t.repo_id
		WHERE t.status = 'ready' AND NOT r.auto_pickup
			AND t.ready_body IS NOT NULL AND t.pickup_blocked = ''`)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	var found []repoBlock
	for rows.Next() {
		var b repoBlock
		if err = rows.Scan(&b.id, &b.repo); err != nil {
			rows.Close()
			return postgres.PgxErrorToHTTPError(err)
		}
		found = append(found, b)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	for _, b := range found {
		reason := "auto pickup is off for " + b.repo
		if _, err = r.AddComment(ctx, b.id, pickupAuthor, "Not picked up: "+reason+
			". Turn it on in the repo settings, or press Run."); err != nil {
			return err
		}
		if _, err = r.BlockPickup(ctx, b.id, skipPrefix+reason); err != nil {
			return err
		}
	}
	return nil
}

// NextReady is the oldest highest-priority Ready task of a repo that has auto pickup
// on, with its approved text. It finds none while the board is paused, a job is
// queued, or the running jobs fill the max_jobs slots of the online runners, so it
// is the only query a quiet claim needs. A task from before
// the Ready snapshots existed has no approved text and is skipped, as is one that
// pickupSkipReason rules out; that reason is stored in pickup_blocked and cleared
// once the task is clear again. None is not found.
func (r *TasksRepository) NextReady(
	ctx context.Context,
) (*models.PickupCandidate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.number, t.repo_id, t.ready_title, t.ready_body, t.labels,
			t.pickup_blocked, r.owner || '/' || r.name
		FROM factory.tasks t
		JOIN factory.repos r ON r.id = t.repo_id
		WHERE t.status = 'ready' AND r.auto_pickup AND t.ready_body IS NOT NULL
			AND (t.pickup_blocked = '' OR t.pickup_blocked LIKE $1 || '%')
			AND NOT COALESCE(
				(SELECT pickup_paused FROM factory.board_settings -- single-row
				), false
			)
			AND NOT EXISTS (SELECT 1 FROM factory.jobs j WHERE j.status = 'queued')
			AND (SELECT count(*) FROM factory.jobs j WHERE j.status = 'running') < (
				SELECT COALESCE(sum(max_jobs), 0) FROM factory.runners
				WHERE paired_at IS NOT NULL AND revoked_at IS NULL
					AND last_seen > now() - make_interval(secs => $2)
			)
		ORDER BY t.priority, t.created_at, t.id`, skipPrefix,
		models.OnlineWindow.Seconds())
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	var cands []readyRow
	for rows.Next() {
		var x readyRow
		if err = rows.Scan(&x.c.TaskID, &x.c.Number, &x.c.RepoID, &x.c.Title,
			&x.c.Body, &x.labels, &x.blocked, &x.repo); err != nil {
			rows.Close()
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		cands = append(cands, x)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	for i := range cands {
		x := cands[i]
		skip, vetErr := r.vet(ctx, x)
		if vetErr != nil {
			return nil, vetErr
		}
		if skip {
			continue
		}
		if x.blocked != "" {
			if _, err = r.BlockPickup(ctx, x.c.TaskID, ""); err != nil {
				return nil, err
			}
		}
		c := x.c
		return &c, nil
	}
	return nil, postgres.PgxErrorToHTTPError(pgx.ErrNoRows)
}

// readyRow is a Ready task NextReady is still vetting.
type readyRow struct {
	c       models.PickupCandidate
	labels  []string
	blocked string
	repo    string
}

// vet reports whether auto pickup must pass the task by, recording why.
func (r *TasksRepository) vet(ctx context.Context, x readyRow) (bool, error) {
	reason, err := r.pickupSkipReason(ctx, x.c.TaskID, x.c.Number, x.labels, x.c.Body)
	if err != nil {
		return false, err
	}
	if reason != "" {
		_, err = r.BlockPickup(ctx, x.c.TaskID, skipPrefix+reason)
		return true, err
	}
	noRunner, err := r.noRunnerReason(ctx, x.c.TaskID, x.repo, x.blocked)
	return noRunner != "", err
}

// noRunnerReason blocks a task whose repo no online runner allows, with a comment
// the first time, and returns the block; "" when some runner allows the repo.
func (r *TasksRepository) noRunnerReason(
	ctx context.Context,
	id uuid.UUID,
	repo, blocked string,
) (string, error) {
	ok, err := repoAllowed(ctx, r.db, repo)
	if err != nil || ok {
		return "", err
	}
	reason := skipPrefix + "no runner allows " + repo
	if blocked == reason {
		return reason, nil
	}
	body := "Not picked up: No runner allows " + repo + ". Allow it on the Mac with " +
		"`factory-runner config repos add " + repo + "`."
	if _, err = r.AddComment(ctx, id, pickupAuthor, body); err != nil {
		return "", err
	}
	_, err = r.BlockPickup(ctx, id, reason)
	return reason, err
}
