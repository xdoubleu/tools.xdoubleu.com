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
	return filepath.Join(InstallDir(homeDir), "paused")
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
	notify func(title, body string)
	// wake nudges the daemon loop out of a held heartbeat when a check is asked for.
	wake chan struct{}
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
		notify:    nil,
		wake:      make(chan struct{}, 1),
	}
}

// SetNotifier sets how Notify reaches the owner; the menu bar does, where the
// platform allows.
func (c *Control) SetNotifier(notify func(title, body string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notify = notify
}

// Notify shows a best-effort notification. Without a notifier, or on a nil
// Control, it does nothing.
func (c *Control) Notify(body string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify("Factory Runner", body)
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

// RecordBeat notes a successful heartbeat. A nil Control ignores it.
func (c *Control) RecordBeat() {
	if c == nil {
		return
	}
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

// RequestCheck asks for a self-update check now, instead of at the next hour,
// and wakes the daemon loop so it doesn't wait out a held heartbeat.
func (c *Control) RequestCheck() {
	c.mu.Lock()
	c.check = true
	c.update = "check requested"
	c.mu.Unlock()
	c.Notify("Checking for updates…")
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// refuseCheck drops a pending check that can't run, saying why in the menu.
func (c *Control) refuseCheck(reason string) {
	c.mu.Lock()
	refused := c.check
	c.check = false
	if refused {
		c.update = reason
	}
	c.mu.Unlock()
	if refused {
		c.Notify(reason)
	}
}

// deferCheck keeps a pending check for later, saying why it hasn't run (once,
// as the loop asks again every pass).
func (c *Control) deferCheck(reason string) {
	c.mu.Lock()
	changed := c.check && c.update != reason
	if changed {
		c.update = reason
	}
	c.mu.Unlock()
	if changed {
		c.Notify(reason)
	}
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
