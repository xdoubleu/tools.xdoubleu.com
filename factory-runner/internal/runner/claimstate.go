package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	statusFileName = "status.json"
	statusPerm     = 0o600
	// StatusFresh is how old the status file may be for its runner to count as
	// alive; the daemon rewrites it every few seconds.
	StatusFresh = 30 * time.Second
	// QueuedUnknown is the queued count before the server has answered one.
	QueuedUnknown = -1
)

// StatusPath is the file the running daemon keeps current for `factory-runner
// status`.
func StatusPath(homeDir string) string {
	return filepath.Join(InstallDir(homeDir), statusFileName)
}

// ClaimState says whether this runner claims jobs and, when it does not, why.
type ClaimState struct {
	// Claiming is true when the loop would take a job now.
	Claiming bool `json:"claiming"`
	// Reason is empty while claiming; otherwise the one cause that holds claims.
	Reason string `json:"reason"`
	// Owner is true when the owner chose the hold (stopped, mode off), which is
	// not a stall.
	Owner    bool `json:"owner"`
	Draining bool `json:"draining"`
	// JobsOff is the error that kept the job loop from starting, empty when on.
	JobsOff string   `json:"jobsOff"`
	Running int      `json:"running"`
	Max     int      `json:"max"`
	Jobs    []string `json:"jobs"`
	// Queued is the server's count of jobs waiting, QueuedUnknown before an answer.
	Queued int `json:"queued"`
}

// noClaim is the claim state before the job loop reports one.
var noClaim = ClaimState{ //nolint:gochecknoglobals // a constant value
	Claiming: false, Reason: "", Owner: false, Draining: false, JobsOff: "",
	Running: 0, Max: 0, Jobs: nil, Queued: QueuedUnknown,
}

// Line is the state on one line: what it does, why, and how many slots are used.
func (s ClaimState) Line() string {
	slots := fmt.Sprintf("%d of %d slots used", s.Running, s.Max)
	switch {
	case s.Claiming:
		return "Claiming (" + slots + ")"
	case s.Owner:
		return s.Reason
	default:
		return "Holding: " + s.Reason + " (" + slots + ")"
	}
}

// StatusReport is what `factory-runner status` prints: the claim state and the
// facts around it.
type StatusReport struct {
	Version       string     `json:"version"`
	Written       time.Time  `json:"written"`
	LastHeartbeat time.Time  `json:"lastHeartbeat"`
	Update        string     `json:"update"`
	Claim         ClaimState `json:"claim"`
}

// Render prints the report for a person. now decides whether the daemon that
// wrote it is still alive.
func (r StatusReport) Render(now time.Time) string {
	var b strings.Builder
	age := now.Sub(r.Written).Round(time.Second)
	if age > StatusFresh {
		fmt.Fprintf(&b, "Runner is not running (status last written %s ago)\n", age)
		return b.String()
	}
	fmt.Fprintf(&b, "Version:    %s\n", r.Version)
	fmt.Fprintf(&b, "Claims:     %s\n", r.Claim.Line())
	if len(r.Claim.Jobs) > 0 {
		fmt.Fprintf(&b, "Jobs:       %s\n", strings.Join(r.Claim.Jobs, ", "))
	}
	if r.Claim.Queued != QueuedUnknown {
		fmt.Fprintf(&b, "Queued:     %d waiting on the board\n", r.Claim.Queued)
	}
	if !r.LastHeartbeat.IsZero() {
		fmt.Fprintf(&b, "Heartbeat:  %s ago\n",
			now.Sub(r.LastHeartbeat).Round(time.Second))
	}
	if r.Update != "" {
		fmt.Fprintf(&b, "Update:     %s\n", r.Update)
	}
	return b.String()
}

// ReadStatus reads the report the daemon wrote.
func ReadStatus(homeDir string) (StatusReport, error) {
	var r StatusReport
	data, err := os.ReadFile(StatusPath(homeDir))
	if err != nil {
		return r, fmt.Errorf("read status: %w", err)
	}
	if err = json.Unmarshal(data, &r); err != nil {
		return r, fmt.Errorf("decode status: %w", err)
	}
	return r, nil
}

// WriteStatus replaces the report file, so a reader never sees half of one.
func WriteStatus(homeDir string, r StatusReport) error {
	path := StatusPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), installDirPerm); err != nil {
		return fmt.Errorf("create status dir: %w", err)
	}
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode status: %w", err)
	}
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, data, statusPerm); err != nil {
		return fmt.Errorf("write status: %w", err)
	}
	return os.Rename(tmp, path) //nolint:wrapcheck // the caller names it
}

// ClaimSources are the live facts the claim state is built from, set once the
// job loop exists.
type ClaimSources struct {
	// Max is how many jobs may run at once.
	Max func() int
	// Jobs are the ids of the jobs running now.
	Jobs func() []string
	// LimitUntil is when a usage limit lifts; zero when none holds.
	LimitUntil func() time.Time
}

// SetClaimSources tells the control where the job loop's numbers come from.
func (c *Control) SetClaimSources(s ClaimSources) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sources = s
}

// SetJobsOff records why the job loop could not start; "" clears it.
func (c *Control) SetJobsOff(reason string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.jobsOff = reason
}

// RecordQueued notes the server's count of waiting jobs from a heartbeat.
func (c *Control) RecordQueued(n int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queued = n
}

// ClaimState is the answer to "why isn't this runner taking jobs".
func (c *Control) ClaimState() ClaimState {
	c.mu.Lock()
	src, jobsOff, draining, restarting := c.sources, c.jobsOff, c.draining, c.restarting
	paused, unpaired, queued, mode := c.paused, c.unpaired, c.queued, c.mode
	c.mu.Unlock()

	s := ClaimState{
		Claiming: false, Reason: "", Owner: false, Draining: draining,
		JobsOff: jobsOff, Running: 0, Max: 1, Jobs: nil, Queued: queued,
	}
	if src.Max != nil {
		s.Max = max(src.Max(), 1)
	}
	if src.Jobs != nil {
		s.Jobs = src.Jobs()
		slices.Sort(s.Jobs)
		s.Running = len(s.Jobs)
	}
	var limit time.Time
	if src.LimitUntil != nil {
		limit = src.LimitUntil()
	}
	// Read outside the lock: the source may itself ask whether the runner is paused.
	off := mode != nil && mode() == ModeOff
	switch {
	case unpaired:
		s.Reason, s.Owner = "not paired", true
	case paused:
		s.Reason, s.Owner = "stopped from the menu", true
	case off:
		s.Reason, s.Owner = "mode is off", true
	case jobsOff != "":
		s.Reason = "jobs off: " + jobsOff
	case restarting:
		s.Reason = "restarting after an update"
	case draining:
		s.Reason = "draining for a newer release"
	case c.now().Before(limit):
		s.Reason = "usage limit until " + limit.Local().Format("15:04")
	case s.Running >= s.Max:
		s.Reason = "all slots busy"
	default:
		s.Claiming = true
	}
	return s
}

// Report builds the status file's content.
func (c *Control) Report() StatusReport {
	snap := c.Snapshot()
	return StatusReport{
		Version: snap.Version, Written: c.now(), LastHeartbeat: snap.LastHeartbeat,
		Update: snap.Update, Claim: c.ClaimState(),
	}
}

// RequestRestart asks the daemon to end with ErrRestart, which a stuck drain
// needs; launchd then starts the runner afresh.
func (c *Control) RequestRestart() {
	select {
	case c.restart <- struct{}{}:
	default:
	}
}

// KickJobs asks a runner whose jobs are off to try starting them now.
func (c *Control) KickJobs() {
	select {
	case c.kick <- struct{}{}:
	default:
	}
}

// JobsKicked is signalled by KickJobs.
func (c *Control) JobsKicked() <-chan struct{} {
	return c.kick
}

// restartRequests is signalled by RequestRestart; a nil Control never is.
func (c *Control) restartRequests() <-chan struct{} {
	if c == nil {
		return nil
	}
	return c.restart
}
