package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

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
	awake Awake
	stop  func()
}

// NewActivity returns an Activity that holds the Mac awake through awake.
func NewActivity(awake Awake) *Activity {
	return &Activity{mu: sync.Mutex{}, n: 0, awake: awake, stop: nil}
}

// Begin records a job starting. A failure to hold the Mac awake is returned
// but doesn't stop the job: it only risks the Mac sleeping.
func (a *Activity) Begin() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.n++
	if a.n != 1 {
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
