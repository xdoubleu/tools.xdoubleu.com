package services

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

const (
	// ParkedLabel marks a Backlog task waiting for a new job image.
	ParkedLabel = "waiting-on-sandbox"
	// imageResumeAuthor creates the jobs that resume parked tasks.
	imageResumeAuthor = "system:job-image"
	// maxImageResumes is how many times a task is resumed on a new image before
	// a sandbox block goes to Needs you.
	maxImageResumes = 2
)

const imageResumePrompt = "The job image has been updated, so the tool or access " +
	"that blocked you may now be there. Retry what was blocked, finish the work, " +
	"and write the outcome."

// DigestSource returns the job image digest currently published.
type DigestSource func(ctx context.Context) (string, error)

// SetImageDigestSource turns on resuming parked tasks when the digest changes.
func (s *JobService) SetImageDigestSource(src DigestSource) { s.imageDigest = src }

// park moves a blocked job's task to Backlog, labelled, instead of Needs you, when
// a new image can resume its session. It leaves the task in Needs you when the job
// has no session or pull request to worry about, or the task was resumed too often.
func (s *JobService) park(
	ctx context.Context,
	job *models.Job,
	sandbox *models.Task,
) error {
	if s.imageDigest == nil || job.SessionID == "" || job.PRURL != "" {
		return nil
	}
	task, err := s.tasks.Get(ctx, job.TaskID)
	if err != nil {
		return err
	}
	if task.Status != models.StatusNeedsYou {
		return nil
	}
	jobs, err := s.jobs.ByTask(ctx, job.TaskID)
	if err != nil {
		return err
	}
	resumes := 0
	for i := range jobs {
		if jobs[i].CreatedBy == imageResumeAuthor {
			resumes++
		}
	}
	if resumes >= maxImageResumes {
		return nil
	}
	if !slices.Contains(task.Labels, ParkedLabel) {
		labels := append(slices.Clone(task.Labels), ParkedLabel)
		if err = s.tasks.SetLabels(ctx, task.ID, labels); err != nil {
			return err
		}
	}
	body := "Parked until a new job image is published; the task then resumes by " +
		"itself."
	if sandbox != nil {
		body += " Waiting on sandbox task T-" +
			strconv.FormatInt(sandbox.Number, 10) + "."
	}
	if err = s.finishTask(
		ctx, task.ID, systemAuthor, body, models.StatusBacklog); err != nil {
		return err
	}
	s.recordFirstDigest(ctx)
	return nil
}

// recordFirstDigest stores the current digest when none is known, so the next
// image resumes a task parked before the periodic job first ran. Best effort.
func (s *JobService) recordFirstDigest(ctx context.Context) {
	if seen, err := s.settings.ImageDigest(ctx); err != nil || seen != "" {
		return
	}
	if digest, err := s.imageDigest(ctx); err == nil {
		_ = s.settings.SetImageDigest(ctx, digest)
	}
}

// ResumeParked queues a job resuming the session of every parked task once the
// published job image digest differs from the one last seen. The first digest
// seen is only recorded. A failure leaves the digest unrecorded, so it retries.
func (s *JobService) ResumeParked(ctx context.Context) (int, error) {
	if s.imageDigest == nil {
		return 0, nil
	}
	digest, err := s.imageDigest(ctx)
	if err != nil {
		return 0, err
	}
	seen, err := s.settings.ImageDigest(ctx)
	if err != nil {
		return 0, err
	}
	if seen == digest {
		return 0, nil
	}
	n := 0
	if seen != "" {
		if n, err = s.resumeAll(ctx); err != nil {
			return n, err
		}
	}
	return n, s.settings.SetImageDigest(ctx, digest)
}

func (s *JobService) resumeAll(ctx context.Context) (int, error) {
	all, err := s.tasks.List(ctx, "")
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for i := range all {
		t := &all[i]
		if (t.Status != models.StatusBacklog && t.Status != models.StatusInProgress) ||
			!slices.Contains(t.Labels, ParkedLabel) {
			continue
		}
		if err = s.resume(ctx, t); err != nil {
			errs = append(errs, err)
			continue
		}
		n++
	}
	return n, errors.Join(errs...)
}

func (s *JobService) resume(ctx context.Context, t *models.Task) error {
	jobs, err := s.jobs.ByTask(ctx, t.ID)
	if err != nil {
		return err
	}
	unparked := slices.DeleteFunc(slices.Clone(t.Labels), func(l string) bool {
		return l == ParkedLabel
	})
	if hasActive(jobs) {
		return s.tasks.SetLabels(ctx, t.ID, unparked)
	}
	// Jobs are newest first: resume the newest session there is.
	i := slices.IndexFunc(jobs, func(j models.Job) bool { return j.SessionID != "" })
	if i < 0 {
		if err = s.tasks.SetLabels(ctx, t.ID, unparked); err != nil {
			return err
		}
		return s.finishTask(ctx, t.ID, systemAuthor,
			"A new job image is published, but there is no session to resume.",
			models.StatusNeedsYou)
	}
	last := jobs[i]
	if _, err = s.jobs.Create(ctx, models.Job{
		ID: uuid.Nil, Kind: "", TaskID: t.ID, TaskNumber: 0, RepoID: last.RepoID,
		Harness: "", Model: "", Prompt: imageResumePrompt,
		Status: "", Branch: last.Branch, SessionID: "", Outcome: "",
		ParentJobID: &last.ID, PRURL: "", RunnerID: nil, PRState: "", PRCI: "",
		PRReview: "", CreatedBy: imageResumeAuthor, CreatedAt: time.Time{},
		Routine: "", Limited: false, LimitResetsAt: nil,
		FinishedHarness: "", FinishedModel: "",
		StartedAt: nil, LastCheckIn: nil, FinishedAt: nil,
	}); err != nil {
		return err
	}
	// The label goes last, so a failure above leaves the task to retry.
	if err = s.tasks.SetLabels(ctx, t.ID, unparked); err != nil {
		return err
	}
	return s.finishTask(ctx, t.ID, systemAuthor,
		"A new job image is published; the task resumes on its branch.",
		models.StatusInProgress)
}
