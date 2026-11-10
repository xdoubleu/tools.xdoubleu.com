package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const pausePerm = 0o600

// PausePath is the flag file whose presence means the owner stopped this runner.
// It outlives the process, so launchd restarting the runner doesn't undo a Stop.
func PausePath(homeDir string) string {
	return filepath.Join(filepath.Dir(InstalledBinaryPath(homeDir)), "paused")
}

// Status is what the menu bar shows.
type Status struct {
	Version       string
	Paused        bool
	Busy          bool
	LastHeartbeat time.Time
	// Update is the outcome of the latest self-update check, empty before the first.
	Update string
}

// Control is the runner's local state shared by the daemon, the claim loop and
// the menu bar: the persisted pause flag and the latest observations.
type Control struct {
	pausePath string
	version   string
	activity  *Activity
	now       func() time.Time

	mu     sync.Mutex
	paused bool
	beat   time.Time
	update string
	check  bool
}

// NewControl reads the pause flag from pausePath.
func NewControl(pausePath, version string, activity *Activity) *Control {
	_, err := os.Stat(pausePath)
	return &Control{
		pausePath: pausePath,
		version:   version,
		activity:  activity,
		now:       time.Now,
		mu:        sync.Mutex{},
		paused:    err == nil,
		beat:      time.Time{},
		update:    "",
		check:     false,
	}
}

// Paused reports whether the owner stopped this runner.
func (c *Control) Paused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

// SetPaused stops or resumes claiming jobs and heartbeating, and persists it.
// A running job still finishes.
func (c *Control) SetPaused(paused bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if paused {
		if err := os.MkdirAll(filepath.Dir(c.pausePath), installDirPerm); err != nil {
			return fmt.Errorf("create pause dir: %w", err)
		}
		if err := os.WriteFile(c.pausePath, nil, pausePerm); err != nil {
			return fmt.Errorf("write pause flag: %w", err)
		}
	} else if err := os.Remove(c.pausePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove pause flag: %w", err)
	}
	c.paused = paused
	return nil
}

// Quiet is whether the heartbeat should stay silent: stopped, and no job left
// to report on.
func (c *Control) Quiet() bool {
	return c.Paused() && (c.activity == nil || c.activity.Idle())
}

// RecordBeat notes a successful heartbeat.
func (c *Control) RecordBeat() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.beat = c.now()
}

// RecordUpdate notes the outcome of a self-update check.
func (c *Control) RecordUpdate(outcome string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.update = outcome
}

// Snapshot returns the current status.
func (c *Control) Snapshot() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Status{
		Version:       c.version,
		Paused:        c.paused,
		Busy:          c.activity != nil && !c.activity.Idle(),
		LastHeartbeat: c.beat,
		Update:        c.update,
	}
}

// Line renders the status as one menu line.
func (s Status) Line() string {
	switch {
	case s.Paused && s.Busy:
		return "Stopping after the current job"
	case s.Paused:
		return "Stopped"
	case s.Busy:
		return "Running a job"
	default:
		return "Idle"
	}
}

// RequestCheck asks for a self-update check now, instead of at the next hour.
func (c *Control) RequestCheck() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.check = true
}

// takeCheck reports and clears a requested check.
func (c *Control) takeCheck() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	requested := c.check
	c.check = false
	return requested
}

// MenuLines are the menu's read-only lines.
type MenuLines struct {
	Header, Status, Heartbeat, Update string
}

const shortRelease = 7

// MenuLines renders the status for the menu bar at time now.
func (s Status) MenuLines(release string, now time.Time) MenuLines {
	version := release
	if len(version) > shortRelease && version != DevRelease {
		version = version[:shortRelease]
	}
	beat := "No heartbeat yet"
	if s.Paused && !s.Busy {
		beat = "Offline on the board"
	} else if !s.LastHeartbeat.IsZero() {
		ago := now.Sub(s.LastHeartbeat).Round(time.Second)
		beat = "Last heartbeat " + ago.String() + " ago"
	}
	update := "No update check yet"
	if s.Update != "" {
		update = "Update: " + s.Update
	}
	return MenuLines{
		Header:    "Factory Runner " + version,
		Status:    s.Line(),
		Heartbeat: beat,
		Update:    update,
	}
}
