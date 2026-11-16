package runner

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const (
	// taskMarker names the file in a job directory that holds its task id.
	taskMarker = "task"
	// sweepEvery is how often the loop sweeps the data directory.
	sweepEvery = time.Hour
	// staleJobAge is how old a job directory with no task marker must be to go.
	staleJobAge = 14 * 24 * time.Hour
)

// CleanJobs deletes job directories nobody will resume: those of tasks the
// server reports as finished (Done, gone, or in Testing with a merged pull
// request) and unmarked ones older than staleJobAge. In-flight jobs and open
// tasks are kept, since a Reply or usage-limit continuation resumes from them.
// The server is untrusted, so it can only name directories the runner asked about.
// A failure is only logged; the next sweep tries again.
func (r *JobRunner) CleanJobs(ctx context.Context) {
	byTask, tasks := r.markedJobs()
	for len(tasks) > 0 {
		batch := tasks[:min(len(tasks), maxSessionQuery)]
		tasks = tasks[len(batch):]
		finished, err := r.API.ListFinishedTasks(ctx, r.Token, batch)
		if err != nil {
			if ctx.Err() == nil {
				r.Log.Warn("could not ask which tasks are finished", "error", err)
			}
			return
		}
		for _, task := range finished {
			if !slices.Contains(batch, task) {
				continue
			}
			for _, id := range byTask[task] {
				if _, busy := r.running.Load(id); !busy {
					r.removeJobDir(id)
				}
			}
		}
	}
}

// markedJobs groups the idle job directories that record a task by that task, and
// deletes the stale unmarked ones.
func (r *JobRunner) markedJobs() (map[string][]string, []string) {
	root := filepath.Join(r.DataDir, "jobs")
	entries, err := os.ReadDir(root)
	if err != nil {
		if !os.IsNotExist(err) {
			r.Log.Warn("could not list the job directories", "error", err)
		}
		return nil, nil
	}
	byTask := map[string][]string{}
	var tasks []string
	for _, entry := range entries {
		id := entry.Name()
		if _, busy := r.running.Load(id); busy || !entry.IsDir() ||
			!jobIDPattern.MatchString(id) {
			continue
		}
		task := r.jobTask(filepath.Join(root, id))
		switch {
		case task == "":
			if r.stale(entry) {
				r.removeJobDir(id)
			}
		case byTask[task] == nil:
			tasks = append(tasks, task)
			fallthrough
		default:
			byTask[task] = append(byTask[task], id)
		}
	}
	return byTask, tasks
}

// jobTask is the task id a job directory records, "" when it has none.
func (r *JobRunner) jobTask(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, taskMarker))
	if err != nil {
		return ""
	}
	if task := string(data); taskIDPattern.MatchString(task) {
		return task
	}
	return ""
}

// stale reports whether an unmarked job directory is old enough to delete.
func (r *JobRunner) stale(entry os.DirEntry) bool {
	info, err := entry.Info()
	return err == nil && r.now().Sub(info.ModTime()) > staleJobAge
}

func (r *JobRunner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// removeJobDir deletes one job's directory; id is a UUID.
func (r *JobRunner) removeJobDir(id string) {
	if err := removeAll(filepath.Join(r.DataDir, "jobs", id)); err != nil {
		r.Log.Warn("could not delete a job directory", "job", id, "error", err)
	}
}
