package runner

import (
	"context"
	"time"
)

// claimTimeout stays above the server's 20 s hold on ClaimJob.
const claimTimeout = 35 * time.Second

// ClaimedJob is a job as the server hands it over. Every field is untrusted: the
// runner validates it before it touches the disk or starts anything.
type ClaimedJob struct {
	ID               string `json:"id"`
	TaskID           string `json:"taskId"`
	RepoOwner        string `json:"repoOwner"`
	RepoName         string `json:"repoName"`
	DefaultBranch    string `json:"defaultBranch"`
	Harness          string `json:"harness"`
	Prompt           string `json:"prompt"`
	Branch           string `json:"branch"`
	SetupCommands    string `json:"setupCommands"`
	TimeLimitMinutes int    `json:"timeLimitMinutes"`
	// MCPClientID and MCPClientSecret are the job's own agent account for the board
	// (empty if the server made none). The secret is a credential like any other.
	MCPClientID     string `json:"mcpClientId"`
	MCPClientSecret string `json:"mcpClientSecret"`
}

// JobAPI is the server side of running jobs.
type JobAPI interface {
	// ClaimJob waits for the oldest queued job; nil means none came in the hold.
	ClaimJob(ctx context.Context, token string) (*ClaimedJob, error)
	// CheckIn reports a running job alive and says whether to stop it.
	CheckIn(ctx context.Context, token, jobID, sessionID string) (bool, error)
	// AppendJobEvents sends a batch of the harness's output lines, already redacted.
	AppendJobEvents(ctx context.Context, token, jobID string, lines []EventLine) error
	CompleteJob(
		ctx context.Context,
		token, jobID string,
		succeeded bool,
		outcome, sessionID string,
	) error
}

type claimJobResponse struct {
	Job *ClaimedJob `json:"job"`
}

type checkInRequest struct {
	JobID     string `json:"jobId"`
	SessionID string `json:"sessionId"`
}

type checkInResponse struct {
	Stop bool `json:"stop"`
}

type appendJobEventsRequest struct {
	JobID string      `json:"jobId"`
	Lines []EventLine `json:"lines"`
}

type completeJobRequest struct {
	JobID     string `json:"jobId"`
	Succeeded bool   `json:"succeeded"`
	Outcome   string `json:"outcome"`
	SessionID string `json:"sessionId"`
}

// ClaimJob implements JobAPI.
func (c *Client) ClaimJob(ctx context.Context, token string) (*ClaimedJob, error) {
	var resp claimJobResponse
	err := c.call(ctx, "ClaimJob", token, claimTimeout, struct{}{}, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Job, nil
}

// CheckIn implements JobAPI.
func (c *Client) CheckIn(
	ctx context.Context,
	token, jobID, sessionID string,
) (bool, error) {
	var resp checkInResponse
	err := c.call(ctx, "CheckIn", token, pairTimeout,
		checkInRequest{JobID: jobID, SessionID: sessionID}, &resp)
	return resp.Stop, err
}

// AppendJobEvents implements JobAPI.
func (c *Client) AppendJobEvents(
	ctx context.Context,
	token, jobID string,
	lines []EventLine,
) error {
	return c.call(ctx, "AppendJobEvents", token, pairTimeout,
		appendJobEventsRequest{JobID: jobID, Lines: lines}, &struct{}{})
}

// CompleteJob implements JobAPI.
func (c *Client) CompleteJob(
	ctx context.Context,
	token, jobID string,
	succeeded bool,
	outcome, sessionID string,
) error {
	return c.call(ctx, "CompleteJob", token, pairTimeout, completeJobRequest{
		JobID: jobID, Succeeded: succeeded, Outcome: outcome, SessionID: sessionID,
	}, &struct{}{})
}
