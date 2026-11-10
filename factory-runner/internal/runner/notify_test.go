package runner_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// toasts records notifications; the update goroutine posts them.
type toasts struct {
	mu   sync.Mutex
	list []string
}

func (n *toasts) post(_, body string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.list = append(n.list, body)
}

func (n *toasts) all() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.list...)
}

func notifyingControl(t *testing.T) (*runner.Control, *toasts) {
	t.Helper()
	control := runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	n := &toasts{mu: sync.Mutex{}, list: nil}
	control.SetNotifier(n.post)
	return control, n
}

// runChecks runs the daemon until the update has finished once, with a check
// requested up front when asked.
func runChecks(
	t *testing.T,
	control *runner.Control,
	upd *fakeUpdater,
	requested bool,
) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	if requested {
		control.RequestCheck()
	}
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	hb := newScripted(func(call int) error {
		if call == 2 {
			upd.waitCalled()
			cancel()
		}
		return nil
	}, cancel)
	_ = runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
		runner.Options{Updater: upd, Activity: nil, Control: control},
		func(context.Context, time.Duration) { time.Sleep(time.Millisecond) },
		func() time.Time { return now })
}

func TestControlNotifyWithoutANotifierDoesNothing(t *testing.T) {
	var nilControl *runner.Control
	nilControl.Notify("x")
	runner.NewControl(filepath.Join(t.TempDir(), "p"), "v", nil).Notify("x")
}

func TestARequestedCheckNotifiesItsOutcome(t *testing.T) {
	control, n := notifyingControl(t)
	runChecks(t, control, newFakeUpdater(false, nil), true)
	assert.Equal(t,
		[]string{"Checking for updates…", "Factory Runner is up to date"}, n.all())
}

func TestARequestedCheckNotifiesAFailure(t *testing.T) {
	control, n := notifyingControl(t)
	runChecks(t, control, newFakeUpdater(false, errors.New("offline")), true)
	assert.Equal(t,
		[]string{"Checking for updates…", "Update failed: offline"}, n.all())
}

func TestAnInstalledUpdateIsAlwaysNotified(t *testing.T) {
	control, n := notifyingControl(t)
	runChecks(t, control, newFakeUpdater(true, nil), false)
	assert.Equal(t, []string{"Updated to a new release; restarting"}, n.all())
}

func TestTheHourlyCheckIsSilentUnlessItFails(t *testing.T) {
	control, n := notifyingControl(t)
	runChecks(t, control, newFakeUpdater(false, nil), false)
	assert.Empty(t, n.all())

	control, n = notifyingControl(t)
	runChecks(t, control, newFakeUpdater(false, errors.New("offline")), false)
	assert.Empty(t, n.all())
}

func TestARefusedCheckSaysWhy(t *testing.T) {
	control, n := notifyingControl(t)
	require.NoError(t, control.SetPaused(true))
	control.RequestCheck()
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	// A stopped idle runner never heartbeats, so the loop's sleep ends the test.
	_ = runner.RunForTest(ctx, newScripted(nil, cancel), "frt_x", newQuiet(),
		runner.Options{Updater: newFakeUpdater(false, nil), Activity: nil, Control: control},
		func(context.Context, time.Duration) { cancel() },
		func() time.Time { return now })
	assert.Equal(t, []string{
		"Checking for updates…", "not checked: the runner is stopped",
	}, n.all())
}

func TestADeferredCheckSaysWhyOnce(t *testing.T) {
	control, n := notifyingControl(t)
	activity := runner.NewActivity(awakeStub())
	require.NoError(t, activity.Begin())
	control.RequestCheck()
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	hb := newScripted(func(call int) error {
		if call == 4 {
			cancel()
		}
		return nil
	}, cancel)
	_ = runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
		runner.Options{
			Updater: newFakeUpdater(false, nil), Activity: activity, Control: control,
		},
		func(context.Context, time.Duration) { time.Sleep(time.Millisecond) },
		func() time.Time { return now })
	assert.Equal(t, []string{
		"Checking for updates…", "waiting for the current job to finish",
	}, n.all())
}
