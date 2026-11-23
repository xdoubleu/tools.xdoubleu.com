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
	// Mode is the config mode ("claude" or "off"); empty when unknown.
	Mode string
	// Draining is true while a newer release waits for running jobs to end.
	Draining bool
	// Unpaired is true until this Mac has a device token.
	Unpaired bool
	// Claim is why this runner does or doesn't claim; the menu shows it when set.
	Claim ClaimState
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
	// draining stops new claims until the pending update has run.
	draining bool
	// restarting is set once an update is installed and only the restart waits.
	restarting bool
	notify     func(title, body string)
	// mode reads the config mode for the menu; nil leaves it unshown.
	mode     func() string
	unpaired bool
	// paired is signalled by MarkPaired, for a runner waiting to be paired.
	paired chan struct{}
	// wake nudges the daemon loop out of a held heartbeat when a check is asked for.
	wake chan struct{}
	// restart and kick carry the watchdog's repairs to the daemon and the retry
	// of the job loop.
	restart chan struct{}
	kick    chan struct{}
	sources ClaimSources
	jobsOff string
	// queued is the server's latest count of waiting jobs.
	queued int
}

// NewControl reads the pause flag from pausePath.
func NewControl(pausePath, version string, activity *Activity) *Control {
	_, err := os.Stat(pausePath)
	return &Control{
		pausePath:  pausePath,
		version:    version,
		activity:   activity,
		now:        time.Now,
		mu:         sync.Mutex{},
		paused:     err == nil,
		beat:       time.Time{},
		update:     "",
		check:      false,
		draining:   false,
		restarting: false,
		notify:     nil,
		mode:       nil,
		unpaired:   false,
		paired:     make(chan struct{}, 1),
		wake:       make(chan struct{}, 1),
		restart:    make(chan struct{}, 1),
		kick:       make(chan struct{}, 1),
		sources:    ClaimSources{Max: nil, Jobs: nil, LimitUntil: nil, Fault: nil},
		jobsOff:    "",
		queued:     QueuedUnknown,
	}
}

// SetUnpaired marks this Mac as not yet paired, which the menu shows.
func (c *Control) SetUnpaired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unpaired = true
}

// MarkPaired records that a device token is now stored and wakes AwaitPaired.
func (c *Control) MarkPaired() {
	c.mu.Lock()
	c.unpaired = false
	c.mu.Unlock()
	select {
	case c.paired <- struct{}{}:
	default:
	}
}

// AwaitPaired is signalled when MarkPaired runs.
func (c *Control) AwaitPaired() <-chan struct{} {
	return c.paired
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
	mode := c.mode
	sourced := c.sources.Max != nil
	s := Status{
		Version:       c.version,
		Paused:        c.paused,
		Busy:          c.activity != nil && !c.activity.Idle(),
		LastHeartbeat: c.beat,
		Update:        c.update,
		Mode:          "",
		Draining:      c.draining,
		Unpaired:      c.unpaired,
		Claim:         noClaim,
	}
	c.mu.Unlock()
	// Read outside the lock: the source may itself ask whether the runner is paused.
	if mode != nil {
		s.Mode = mode()
	}
	if sourced {
		s.Claim = c.ClaimState()
	}
	return s
}

// Line renders the status as one menu line.
func (s Status) Line() string {
	switch {
	case s.Unpaired:
		return "Not paired"
	case s.Paused && s.Busy:
		return "Stopping after the current job"
	case s.Paused:
		return "Stopped"
	case s.Draining && s.Busy:
		return "Updating after current jobs"
	case s.Busy:
		return "Running a job"
	case s.Mode == ModeOff:
		return "Not claiming: mode is off"
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

// SetDraining records whether a newer release is waiting for running jobs.
func (c *Control) SetDraining(draining bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.draining = draining
}

// Draining reports whether claims are held for a pending update.
func (c *Control) Draining() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.draining
}

// SetRestarting stops new claims until the process restarts.
func (c *Control) SetRestarting() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.restarting = true
}

// Restarting reports whether an installed update waits for the restart.
func (c *Control) Restarting() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.restarting
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
	status := s.Line()
	if s.Claim.Max > 0 && !s.Paused && !s.Unpaired {
		status = s.Claim.Line()
	}
	return MenuLines{
		Header:    "Factory Runner " + version,
		Status:    status,
		Heartbeat: beat,
		Update:    update,
	}
}

// SetModeSource sets how the status reads the config mode.
func (c *Control) SetModeSource(mode func() string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mode = mode
}
