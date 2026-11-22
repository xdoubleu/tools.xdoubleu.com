package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// pollBatch is how many open pull requests one poll reads, the longest unread
// first: a batch that is too big for GitHub's rate limit would starve the rest.
const pollBatch = 50

// PollPullRequests reads where each open pull request of a job stands, keeps it
// on the job for the board, and moves the task when the pull request ends: a
// merge sends a task that is In review to Done, and a close without a merge back
// to Needs you. It does nothing without a GitHub App. One failing pull request
// doesn't stop the others; the errors come back together.
func (s *JobService) PollPullRequests(ctx context.Context) (int, error) {
	if s.github == nil {
		return 0, nil
	}
	jobs, err := s.jobs.WithOpenPullRequest(ctx, pollBatch)
	if err != nil {
		return 0, err
	}
	var failures []error
	for i := range jobs {
		if err = s.pollPullRequest(ctx, &jobs[i]); err != nil {
			failures = append(failures, fmt.Errorf("job %s: %w", jobs[i].ID, err))
			// Take its turn at the back: a few that always fail must not starve the
			// healthy ones of every batch.
			_ = s.jobs.MarkPullRequestPolled(ctx, jobs[i].ID)
		}
	}
	moved, err := s.mergedLinked(ctx)
	if err != nil {
		failures = append(failures, err)
	}
	if err = s.syncLinked(ctx); err != nil {
		failures = append(failures, err)
	}
	if _, err = s.BackfillPathRules(ctx); err != nil {
		failures = append(failures, err)
	}
	return len(jobs) + moved, errors.Join(failures...)
}

// taskLine is the `Task: <title> (<id>)` line a pull request body links its task
// with.
// The id is T-<number>, or a UUID in pull requests opened before numbers.
var taskLine = regexp.MustCompile(
	`(?m)^Task: .*\((T-[0-9]+|[0-9a-fA-F-]{36})\)\s*$`)

// mergedLinked moves the task of each recently merged pull request
// that links one, whoever opened it. It leaves a task that has jobs (their own
// pull requests are polled above), one a person has moved on, and one that is
// not In review. A repeat does nothing, as the task has moved. The App's webhook
// does the same per merge; this catches a delivery that never arrived.
func (s *JobService) mergedLinked(ctx context.Context) (int, error) {
	repos, err := s.repos.List(ctx)
	if err != nil {
		return 0, err
	}
	var failures []error
	var pulls []PullRequestRef
	var refs []string
	for i := range repos {
		listed, listErr := skipNotInstalled(
			s.github.MergedPullRequests(ctx, repos[i].Owner, repos[i].Name),
		)
		if listErr != nil {
			failures = append(failures, fmt.Errorf("repo %s: %w", repos[i].Name, listErr))
			continue
		}
		for _, p := range listed {
			pr := PullRequestRef(p)
			pulls = append(pulls, pr)
			refs = append(refs, linkedTasks(pr)...)
		}
	}
	// One query for every task of every merged pull request, however many.
	lookup, err := s.lookupTasks(ctx, refs)
	if err != nil {
		return 0, errors.Join(append(failures, err)...)
	}
	moved := 0
	var rows []models.LinkedPullRequest
	seen := map[string]bool{}
	for _, pr := range pulls {
		for _, task := range lookup.tasksOf(linkedTasks(pr)) {
			if key := pr.URL + task.ID.String(); !seen[key] {
				seen[key] = true
				rows = append(rows, mergedRow(pr.URL, task.ID))
			}
		}
	}
	if err = s.linked.UpsertMany(ctx, rows); err != nil {
		return 0, errors.Join(append(failures, err)...)
	}
	for _, pr := range pulls {
		done, moveErr := s.mergeTasks(ctx, pr, lookup, false)
		if moveErr != nil {
			failures = append(failures, fmt.Errorf("pull request %s: %w", pr.URL, moveErr))
		}
		if done {
			moved++
		}
	}
	return moved, errors.Join(failures...)
}

func mergedRow(url string, taskID uuid.UUID) models.LinkedPullRequest {
	return models.LinkedPullRequest{
		URL: url, TaskID: taskID, State: StateMerged,
		CI: "", Review: "", Mergeable: "", UnmergeableSince: nil,
		CheckedAt: time.Time{},
	}
}

// skipNotInstalled turns "the App is not installed on the repo" into an empty
// listing: a repo it cannot reach is not a failure of the poll over the others.
func skipNotInstalled[T any](pulls []T, err error) ([]T, error) {
	if errors.Is(err, ErrNotInstalled) {
		return nil, nil
	}
	return pulls, err
}

// PullRequestRef is what a pull request offers to link tasks from.
type PullRequestRef struct {
	URL, Title, Branch, Body, MergeSHA string
}

// taskID is a task written T-<number>.
var taskID = regexp.MustCompile(`\bT-[0-9]+\b`)

// titleTask is the task a title names in parentheses, such as `Fix it (T-123)`.
var titleTask = regexp.MustCompile(`\((T-[0-9]+)\)`)

// branchTask is the task a branch is named for, such as agent/t483-fix.
var branchTask = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])t-?([0-9]+)(?:[^0-9]|$)`)

// bodyTaskList is a closing keyword (`Closes T-1, T-2`) or a `Tasks:` line
// followed by a list of tasks. A task mentioned in prose is not one.
var bodyTaskList = regexp.MustCompile(
	`(?im)(?:\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?|part of|refs?)\s+|^Tasks:\s*)` +
		`(T-[0-9]+(?:(?:\s*,\s*|\s*&\s*|\s+and\s+|\s+)T-[0-9]+)*)`)

// linkedTasks is every reference (T-<number> or UUID) a pull request links, each
// once, in order: a `Task:` line of the body, the task in its title, its branch,
// then the tasks a closing keyword or `Tasks:` line of the body lists.
func linkedTasks(pr PullRequestRef) []string {
	var refs []string
	seen := map[string]bool{}
	add := func(ref string) {
		if !seen[ref] {
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	for _, m := range taskLine.FindAllStringSubmatch(pr.Body, -1) {
		add(m[1])
	}
	for _, m := range titleTask.FindAllStringSubmatch(pr.Title, -1) {
		add(m[1])
	}
	if m := branchTask.FindStringSubmatch(pr.Branch); m != nil {
		add("T-" + m[1])
	}
	for _, m := range bodyTaskList.FindAllStringSubmatch(pr.Body, -1) {
		for _, id := range taskID.FindAllString(m[1], -1) {
			add(id)
		}
	}
	return refs
}

// early reports whether a task is waiting to be worked on or being worked on
// outside a job: a pull request for it moves it on.
func early(status models.Status) bool {
	return status == models.StatusReady || status == models.StatusInProgress ||
		status == models.StatusNeedsYou
}

// PullRequestOpened links each task an open pull request names and moves one
// that is waiting or in progress, with no job of its own, to In review.
func (s *JobService) PullRequestOpened(ctx context.Context, pr PullRequestRef) error {
	var failures []error
	for _, ref := range linkedTasks(pr) {
		id, err := s.tasks.Resolve(ctx, ref)
		if err != nil {
			continue
		}
		err = s.linked.Upsert(ctx, models.LinkedPullRequest{
			URL: pr.URL, TaskID: id, State: StateOpen,
			CI: "", Review: "", Mergeable: "", UnmergeableSince: nil,
			CheckedAt: time.Time{},
		})
		if err == nil {
			err = s.reviewOpen(ctx, id, pr.URL)
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// reviewOpen moves a task with no jobs that is early to In review, for its open
// pull request.
func (s *JobService) reviewOpen(ctx context.Context, id uuid.UUID, url string) error {
	task, err := s.tasks.Get(ctx, id)
	if err != nil {
		return nil //nolint:nilerr // a task that is gone is not a failure
	}
	return s.reviewOpenTask(ctx, task, url)
}

func (s *JobService) reviewOpenTask(
	ctx context.Context,
	task *models.Task,
	url string,
) error {
	id := task.ID
	if !early(task.Status) {
		return nil
	}
	jobs, err := s.jobs.ByTask(ctx, id)
	if err != nil || len(jobs) > 0 {
		return err
	}
	if _, err = s.tasks.Move(ctx, id, models.StatusInReview); err != nil {
		return err
	}
	_, err = s.tasks.AddComment(ctx, id, systemAuthor, "A pull request was opened: "+url)
	return err
}

// PullRequestClosed notes on each task a pull request names, that has no job of
// its own, that the pull request closed without merging. The task stays put.
func (s *JobService) PullRequestClosed(ctx context.Context, pr PullRequestRef) error {
	var failures []error
	for _, ref := range linkedTasks(pr) {
		id, err := s.tasks.Resolve(ctx, ref)
		if err != nil {
			continue
		}
		jobs, err := s.jobs.ByTask(ctx, id)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if len(jobs) > 0 {
			continue
		}
		body := "The pull request was closed without merging: " + pr.URL
		if _, err = s.tasks.AddComment(ctx, id, systemAuthor, body); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// afterMergeStatus is Testing when the merged pull request adds or changes a
// decision record, whatever the labels, for the owner to review. Otherwise it is
// Done for work the agents found and fixed themselves (the routine-found,
// factory-retro, factory-sandbox and routine-feedback labels), or whose files only
// CI or the board checks (see selfCheckedBy), and Testing for everything else.
// The note names the decision records, or is "".
func (s *JobService) afterMergeStatus(
	ctx context.Context,
	task *models.Task,
	prURL string,
) (models.Status, string) {
	files := s.mergedFiles(ctx, prURL)
	if adrs := adrFiles(files); len(adrs) > 0 {
		return models.StatusTesting, "It adds or changes " + strings.Join(adrs, ", ") +
			": review the decision."
	}
	for _, label := range task.Labels {
		switch strings.ToLower(label) {
		case "routine-found", "factory-retro", "factory-sandbox", "routine-feedback":
			return models.StatusDone, ""
		}
	}
	if selfCheckedBy(files) != "" {
		return models.StatusDone, ""
	}
	return models.StatusTesting, ""
}

// TestingAfterMerge moves each task a merged pull request links in its
// body to Testing, or Done (see afterMergeStatus), under the same rules as the
// poll, and keeps the merge commit for the deploy state. It reports whether a
// task moved.
func (s *JobService) TestingAfterMerge(
	ctx context.Context,
	pr PullRequestRef,
) (bool, error) {
	lookup, err := s.lookupTasks(ctx, linkedTasks(pr))
	if err != nil {
		return false, err
	}
	return s.mergeTasks(ctx, pr, lookup, true)
}

// mergeTasks moves each task of a merged pull request found in lookup. It records
// the pull request as merged on each when upsert is set; the poll does that for
// all of its pull requests in one statement instead.
func (s *JobService) mergeTasks(
	ctx context.Context,
	pr PullRequestRef,
	lookup taskLookup,
	upsert bool,
) (bool, error) {
	var failures []error
	moved := false
	for _, task := range lookup.tasksOf(linkedTasks(pr)) {
		done, err := s.testingAfterMerge(ctx, task, pr, upsert)
		if err != nil {
			failures = append(failures, err)
		}
		moved = moved || done
	}
	return moved, errors.Join(failures...)
}

func (s *JobService) testingAfterMerge(
	ctx context.Context,
	task *models.Task,
	pr PullRequestRef,
	upsert bool,
) (bool, error) {
	id := task.ID
	var err error
	// Also backfills a task that merged without its commit being recorded.
	if task.MergeSHA == "" && pr.MergeSHA != "" {
		err = s.tasks.SetMergeSHA(ctx, id, pr.MergeSHA)
	}
	if err == nil && upsert {
		err = s.linked.Upsert(ctx, mergedRow(pr.URL, id))
	}
	if err != nil {
		return false, err
	}
	if task.Status != models.StatusInReview && !early(task.Status) {
		return false, nil
	}
	jobs, err := s.jobs.ByTask(ctx, id)
	if err != nil {
		return false, err
	}
	if len(jobs) > 0 {
		return false, nil
	}
	to, note := s.afterMergeStatus(ctx, task, pr.URL)
	if _, err = s.tasks.Move(ctx, id, to); err != nil {
		return false, err
	}
	body := "The pull request was merged: " + pr.URL + withNote(note)
	_, err = s.tasks.AddComment(ctx, id, systemAuthor, body)
	return true, err
}

func withNote(note string) string {
	if note == "" {
		return ""
	}
	return " " + note
}

func (s *JobService) pollPullRequest(ctx context.Context, job *models.Job) error {
	repo, err := s.repos.Get(ctx, job.RepoID)
	if err != nil {
		return err
	}
	number, ok := pullNumber(job.PRURL, repo)
	if !ok {
		return errors.New("its pull request address is not this repo's")
	}
	// A pull request can't be deleted, only closed, so "not found" means the App can
	// no longer see it (a rename, a transfer, changed access): that is a failure to
	// look into, never a reason to call the pull request closed.
	status, err := s.github.PullRequest(ctx, repo.Owner, repo.Name, number)
	if err != nil {
		return err
	}
	if status.State != job.PRState && status.State != StateOpen {
		// First the task, then the record: a failure in between repeats this step
		// on the next poll, and the task's own status keeps it from happening twice.
		if err = s.pullRequestEnded(ctx, job, status); err != nil {
			return err
		}
	}
	return s.jobs.SetPullRequestStatus(ctx, job.ID, status.State, status.CI, status.Review)
}

// pullRequestEnded moves the job's task, but only if it is still waiting in In
// review and this is its newest job: a task a person has moved on, or one a later
// job is working on, is not ours to move.
func (s *JobService) pullRequestEnded(
	ctx context.Context,
	job *models.Job,
	status PullRequestStatus,
) error {
	task, err := s.tasks.Get(ctx, job.TaskID)
	if err != nil {
		return err
	}
	if task.Status != models.StatusInReview {
		return nil
	}
	jobs, err := s.jobs.ByTask(ctx, job.TaskID)
	if err != nil {
		return err
	}
	if len(jobs) > 0 && jobs[0].ID != job.ID {
		return nil
	}
	to, suffix := models.StatusNeedsYou, ""
	body := "The pull request was closed without merging: "
	if status.State == StateMerged {
		var note string
		to, note = s.afterMergeStatus(ctx, task, job.PRURL)
		body = "The pull request was merged: "
		suffix = withNote(note)
		err = s.tasks.SetMergeSHA(ctx, job.TaskID, status.MergeSHA)
	}
	// Move first: a failure after it loses a comment, where a failure before it
	// would repeat the comment on every poll until the move worked.
	if err == nil {
		_, err = s.tasks.Move(ctx, job.TaskID, to)
	}
	if err != nil {
		return err
	}
	_, err = s.tasks.AddComment(ctx, job.TaskID, systemAuthor, body+job.PRURL+suffix)
	return err
}

// taskLookup finds the tasks a set of references (T-<number> or UUID) names with
// one query. A reference to a task that is gone finds nothing.
type taskLookup struct {
	byID     map[uuid.UUID]*models.Task
	byNumber map[int64]*models.Task
}

func (s *JobService) lookupTasks(
	ctx context.Context,
	refs []string,
) (taskLookup, error) {
	var ids []uuid.UUID
	var numbers []int64
	for _, ref := range refs {
		if id, err := uuid.Parse(ref); err == nil {
			ids = append(ids, id)
		} else if n, ok := models.ParseTaskRef(ref); ok {
			numbers = append(numbers, n)
		}
	}
	l := taskLookup{
		byID: map[uuid.UUID]*models.Task{}, byNumber: map[int64]*models.Task{},
	}
	if len(ids)+len(numbers) == 0 {
		return l, nil
	}
	tasks, err := s.tasks.ByKeys(ctx, ids, numbers)
	if err != nil {
		return l, err
	}
	for i := range tasks {
		l.byID[tasks[i].ID] = &tasks[i]
		l.byNumber[tasks[i].Number] = &tasks[i]
	}
	return l, nil
}

// tasksOf is the distinct tasks that exist among refs, in order.
func (l taskLookup) tasksOf(refs []string) []*models.Task {
	var tasks []*models.Task
	seen := map[uuid.UUID]bool{}
	for _, ref := range refs {
		if task := l.find(ref); task != nil && !seen[task.ID] {
			seen[task.ID] = true
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (l taskLookup) find(ref string) *models.Task {
	if id, err := uuid.Parse(ref); err == nil {
		return l.byID[id]
	}
	if n, ok := models.ParseTaskRef(ref); ok {
		return l.byNumber[n]
	}
	return nil
}

// syncLinked keeps the status of the open pull requests that link a task in their
// body, whoever opened them: those open now, then any it knew as open that have
// left the list (merged or closed since). The task reads and the writes are one
// query each, however many pull requests there are; only GitHub is asked per
// pull request.
func (s *JobService) syncLinked(ctx context.Context) error {
	repos, err := s.repos.List(ctx)
	if err != nil {
		return err
	}
	open, failures := s.listOpenPulls(ctx, repos)
	var allRefs []string
	for _, o := range open {
		allRefs = append(allRefs, o.refs...)
	}
	lookup, err := s.lookupTasks(ctx, allRefs)
	if err != nil {
		return errors.Join(append(failures, err)...)
	}

	seen := map[string]bool{}
	var rows []models.LinkedPullRequest
	var toReview []linkedReview
	for _, o := range open {
		oRows, oReview, readErr := s.readOpenPull(ctx, o, lookup)
		if readErr != nil {
			failures = append(failures, fmt.Errorf("pull request %s: %w", o.pull.URL, readErr))
		}
		if len(oRows) > 0 || readErr != nil {
			seen[o.pull.URL] = true
		}
		rows = append(rows, oRows...)
		toReview = append(toReview, oReview...)
	}
	staleRows, err := s.refreshStale(ctx, repos, seen)
	if err != nil {
		failures = append(failures, err)
	}
	if err = s.linked.UpsertMany(ctx, append(rows, staleRows...)); err != nil {
		return errors.Join(append(failures, err)...)
	}
	for _, r := range toReview {
		if err = s.reviewOpenTask(ctx, r.task, r.url); err != nil {
			failures = append(failures, fmt.Errorf("pull request %s: %w", r.url, err))
		}
	}
	return errors.Join(failures...)
}

// openPull is an open pull request and the tasks it names.
type openPull struct {
	repo models.Repo
	pull OpenPullRequest
	refs []string
}

// listOpenPulls lists the open pull requests of every repo; a repo it cannot
// list is a failure, not a stop.
func (s *JobService) listOpenPulls(
	ctx context.Context,
	repos []models.Repo,
) ([]openPull, []error) {
	var open []openPull
	var failures []error
	for i := range repos {
		pulls, err := skipNotInstalled(
			s.github.OpenPullRequests(ctx, repos[i].Owner, repos[i].Name),
		)
		if err != nil {
			failures = append(failures, fmt.Errorf("repo %s: %w", repos[i].Name, err))
			continue
		}
		for _, p := range pulls {
			refs := linkedTasks(PullRequestRef{
				URL: p.URL, Title: p.Title, Branch: p.Branch, Body: p.Body, MergeSHA: "",
			})
			open = append(open, openPull{repo: repos[i], pull: p, refs: refs})
		}
	}
	return open, failures
}

// readOpenPull reads an open pull request once from GitHub and returns a row
// for each distinct task of it that exists.
func (s *JobService) readOpenPull(
	ctx context.Context,
	o openPull,
	lookup taskLookup,
) ([]models.LinkedPullRequest, []linkedReview, error) {
	var tasks []*models.Task
	seen := map[uuid.UUID]bool{}
	for _, ref := range o.refs {
		if task := lookup.find(ref); task != nil && !seen[task.ID] {
			seen[task.ID] = true
			tasks = append(tasks, task)
		}
	}
	if len(tasks) == 0 {
		return nil, nil, nil
	}
	status, err := s.github.PullRequest(ctx, o.repo.Owner, o.repo.Name, o.pull.Number)
	if err != nil {
		return nil, nil, err
	}
	rows := make([]models.LinkedPullRequest, len(tasks))
	reviews := make([]linkedReview, len(tasks))
	for i, task := range tasks {
		rows[i] = linkedRow(o.pull.URL, task.ID, status)
		reviews[i] = linkedReview{task: task, url: o.pull.URL}
	}
	return rows, reviews, nil
}

type linkedReview struct {
	task *models.Task
	url  string
}

func linkedRow(
	url string,
	taskID uuid.UUID,
	status PullRequestStatus,
) models.LinkedPullRequest {
	return models.LinkedPullRequest{
		URL: url, TaskID: taskID, State: status.State, CI: status.CI,
		Review: status.Review, Mergeable: status.Mergeable, UnmergeableSince: nil,
		CheckedAt: time.Time{},
	}
}

// refreshStale re-reads the pull requests known as open that the open lists no
// longer showed: they were merged or closed since. It returns their new rows.
func (s *JobService) refreshStale(
	ctx context.Context,
	repos []models.Repo,
	seen map[string]bool,
) ([]models.LinkedPullRequest, error) {
	stale, err := s.linked.Open(ctx, pollBatch)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for _, p := range stale {
		if !seen[p.URL] {
			ids = append(ids, p.TaskID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	tasks, err := s.tasks.ByKeys(ctx, ids, nil)
	if err != nil {
		return nil, err
	}
	taskByID := map[uuid.UUID]*models.Task{}
	for i := range tasks {
		taskByID[tasks[i].ID] = &tasks[i]
	}
	repoByID := map[uuid.UUID]*models.Repo{}
	for i := range repos {
		repoByID[repos[i].ID] = &repos[i]
	}
	var failures []error
	var rows []models.LinkedPullRequest
	for _, p := range stale {
		task := taskByID[p.TaskID]
		if seen[p.URL] || task == nil || repoByID[task.RepoID] == nil {
			continue
		}
		repo := repoByID[task.RepoID]
		number, ok := pullNumber(p.URL, repo)
		if !ok {
			continue
		}
		status, readErr := s.github.PullRequest(ctx, repo.Owner, repo.Name, number)
		if readErr != nil {
			failures = append(failures, fmt.Errorf("pull request %s: %w", p.URL, readErr))
			continue
		}
		rows = append(rows, linkedRow(p.URL, p.TaskID, status))
	}
	return rows, errors.Join(failures...)
}

// PullRequestsOf lists every pull request of a task: those its jobs opened, then
// the others that link it in their body.
func (s *JobService) PullRequestsOf(
	ctx context.Context,
	taskID uuid.UUID,
) ([]models.LinkedPullRequest, error) {
	jobs, err := s.jobs.ByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	linked, err := s.linked.ByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	// A job's pull request is also read as a linked one: that read has its
	// mergeable state.
	mergeable := map[string]string{}
	for _, p := range linked {
		mergeable[p.URL] = p.Mergeable
	}
	out := []models.LinkedPullRequest{}
	seen := map[string]bool{}
	for i := range jobs {
		if jobs[i].PRURL == "" || seen[jobs[i].PRURL] {
			continue
		}
		seen[jobs[i].PRURL] = true
		out = append(out, models.LinkedPullRequest{
			URL: jobs[i].PRURL, TaskID: taskID, State: jobs[i].PRState,
			CI: jobs[i].PRCI, Review: jobs[i].PRReview,
			Mergeable:        mergeable[jobs[i].PRURL],
			UnmergeableSince: nil, CheckedAt: time.Time{},
		})
	}
	for _, p := range linked {
		if !seen[p.URL] {
			out = append(out, p)
		}
	}
	return out, nil
}
