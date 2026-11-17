package obs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"unicode/utf8"

	"connectrpc.com/connect/v2"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/platform/github"
)

const (
	// jobLogTailBytes caps each job's returned log to its tail, where the
	// failure is.
	jobLogTailBytes = 6000
	// maxLogJobs caps the failed jobs fetched per call.
	maxLogJobs = 5

	conclusionFailure = "failure"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func (h *obsConnectHandler) GetWorkflowRunLogs(
	ctx context.Context,
	req *observabilityv1.GetWorkflowRunLogsRequest,
) (*observabilityv1.GetWorkflowRunLogsResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	return h.workflowRunLogs(ctx, req.GetRunId(), int64(req.GetPrNumber()), req.GetJob())
}

// workflowRunLogs returns the log tail of each failed job of run runID, or of
// the failed runs of PR prNumber; job filters by name substring.
func (h *obsConnectHandler) workflowRunLogs(
	ctx context.Context, runID, prNumber int64, job string,
) (*observabilityv1.GetWorkflowRunLogsResponse, error) {
	resp := &observabilityv1.GetWorkflowRunLogsResponse{
		Jobs:       []*observabilityv1.WorkflowJobLog{},
		Configured: true,
	}
	if runID == 0 && prNumber == 0 {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, "run_id or pr_number is required",
		)
	}

	runIDs, err := h.logRunIDs(ctx, runID, prNumber)
	if err != nil {
		return h.logsFailure(ctx, resp, err)
	}

	var errs []error
	for _, id := range runIDs {
		jobs, jobsErr := h.deps.GithubClient.ListWorkflowRunJobs(ctx, id)
		if jobsErr != nil {
			errs = append(errs, jobsErr)
			continue
		}
		for _, j := range jobs {
			if len(resp.Jobs) >= maxLogJobs {
				break
			}
			if !jobFailed(j) || !jobMatches(j.Name, job) {
				continue
			}
			text, logErr := h.deps.GithubClient.GetJobLogs(ctx, j.ID)
			if logErr != nil {
				errs = append(errs, fmt.Errorf("job %q: %w", j.Name, logErr))
				continue
			}
			tail, truncated := tailLog(text, jobLogTailBytes)
			resp.Jobs = append(resp.Jobs, &observabilityv1.WorkflowJobLog{
				RunId:      id,
				Job:        j.Name,
				Conclusion: j.Conclusion,
				Log:        tail,
				Truncated:  truncated,
			})
		}
	}
	if len(errs) > 0 {
		return h.logsFailure(ctx, resp, errors.Join(errs...))
	}
	return resp, nil
}

// logsFailure maps an unset GitHub connection to configured=false and any
// other failure to resp.Error, keeping the logs fetched so far.
func (h *obsConnectHandler) logsFailure(
	ctx context.Context,
	resp *observabilityv1.GetWorkflowRunLogsResponse,
	err error,
) (*observabilityv1.GetWorkflowRunLogsResponse, error) {
	if errors.Is(err, github.ErrNotConfigured) {
		resp.Configured = false
		return resp, nil
	}
	h.deps.Logger.WarnContext(ctx, "workflow run logs unavailable",
		slog.Any("error", err))
	resp.Error = err.Error()
	return resp, nil
}

// logRunIDs is runID, or the failed pull_request runs of prNumber.
func (h *obsConnectHandler) logRunIDs(
	ctx context.Context, runID, prNumber int64,
) ([]int64, error) {
	if runID != 0 {
		return []int64{runID}, nil
	}
	runs, err := h.deps.GithubClient.ListWorkflowRuns(ctx)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, r := range runs {
		if r.Event == "pull_request" && r.PRNumber == prNumber &&
			r.Conclusion == conclusionFailure {
			ids = append(ids, r.ID)
		}
	}
	return ids, nil
}

func jobFailed(j github.WorkflowJob) bool {
	return j.Conclusion == conclusionFailure || j.Conclusion == "timed_out"
}

func jobMatches(name, filter string) bool {
	return filter == "" ||
		strings.Contains(strings.ToLower(name), strings.ToLower(filter))
}

// tailLog strips ANSI escapes and keeps the last max bytes, starting at a
// line boundary.
func tailLog(text string, limit int) (string, bool) {
	text = strings.TrimSpace(ansiEscape.ReplaceAllString(text, ""))
	if len(text) <= limit {
		return text, false
	}
	tail := text[len(text)-limit:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	}
	for !utf8.ValidString(tail) && tail != "" {
		tail = tail[1:]
	}
	return tail, true
}
