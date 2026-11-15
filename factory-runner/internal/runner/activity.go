package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

// DrainTimeout is how long a pending update waits for jobs with no VM running
// before it restarts the runner anyway.
const DrainTimeout = 30 * time.Minute

// Awake keeps the Mac from sleeping until the returned stop is called.
type Awake interface {
	Start() (stop func(), err error)
}

// Caffeinate holds `caffeinate -is -w <pid>`: no idle or system sleep while the
// runner lives. It only keeps a closed lid awake on AC power with an external
// display; the board says so.
type Caffeinate struct {
	start func(pid int) (func(), error)
}

// NewCaffeinate returns the real thing.
func NewCaffeinate() *Caffeinate {
	return &Caffeinate{start: startCaffeinate}
}

func startCaffeinate(pid int) (func(), error) {
	//nolint:gosec // fixed arguments, the pid is our own
	cmd := exec.CommandContext(
		context.Background(), "caffeinate", "-is", "-w", strconv.Itoa(pid),
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start caffeinate: %w", err)
	}
	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}, nil
}

// Start begins holding the Mac awake.
func (c *Caffeinate) Start() (func(), error) {
	return c.start(os.Getpid())
}

// Activity counts the jobs running now. The Mac is held awake while any runs,
// and a self-update waits until none does.
type Activity struct {
	mu    sync.Mutex
	n     int
	vms   int
	awake Awake
	stop  func()
	// drainAt is when the runner began draining for an update; zero when not.
	drainAt time.Time
	now     func() time.Time
}

// NewActivity returns an Activity that holds the Mac awake through awake.
func NewActivity(awake Awake) *Activity {
	return &Activity{
		mu: sync.Mutex{}, n: 0, vms: 0, awake: awake, stop: nil,
		drainAt: time.Time{}, now: time.Now,
	}
}

// Begin records a job starting; the caller must still call End, whatever it
// returns. A failure to hold the Mac awake is returned but doesn't stop the job
// (it only risks the Mac sleeping), and the next Begin tries again.
func (a *Activity) Begin() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.n++
	if a.stop != nil {
		return nil
	}
	stop, err := a.awake.Start()
	if err != nil {
		return err
	}
	a.stop = stop
	return nil
}

// End records a job finishing.
func (a *Activity) End() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.n == 0 {
		return
	}
	a.n--
	if a.n == 0 && a.stop != nil {
		a.stop()
		a.stop = nil
	}
}

// Idle reports whether no job is running.
func (a *Activity) Idle() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.n == 0
}

// BeginVM records a job's VM starting; the caller must call EndVM.
func (a *Activity) BeginVM() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.vms++
}

// EndVM records a job's VM closing.
func (a *Activity) EndVM() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vms > 0 {
		a.vms--
	}
}

// SetDraining records that an update is waiting for jobs to end (or not), so
// DrainIdle can time it out.
func (a *Activity) SetDraining(draining bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case !draining:
		a.drainAt = time.Time{}
	case a.drainAt.IsZero():
		a.drainAt = a.now()
	}
}

// DrainIdle is Idle for a pending update: it also holds once draining has lasted
// DrainTimeout with no VM running, so a job counted busy by mistake cannot hold
// the update forever.
func (a *Activity) DrainIdle() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.n == 0 {
		return true
	}
	return !a.drainAt.IsZero() && a.vms == 0 &&
		a.now().Sub(a.drainAt) >= DrainTimeout
}

// Busy describes what is counted as running, for the log.
func (a *Activity) Busy() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return fmt.Sprintf("%d jobs counted, %d VMs running", a.n, a.vms)
}

// WithClock reads time from now, for tests.
func (a *Activity) WithClock(now func() time.Time) *Activity {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.now = now
	return a
}
