package sentryapi

import (
	"context"
	"errors"
	"fmt"
	"time"

	"tools.xdoubleu.com/internal/oauthconn"
)

// Monitor is the Cron Monitor status of one background job (TrackedJob
// upserts one monitor per job slug via its Sentry check-ins).
type Monitor struct {
	Slug string
	// Status is Sentry's monitor-environment status: "ok", "error",
	// "missed_checkin", "timeout", "active" (no check-in yet), or
	// "disabled".
	Status string
	// LastCheckIn / NextCheckIn are zero when the monitor has never checked
	// in, or when Sentry reports no environment for it yet.
	LastCheckIn time.Time
	NextCheckIn time.Time
}

type monitorWire struct {
	Slug         string               `json:"slug"`
	Status       string               `json:"status"`
	Environments []monitorEnvironment `json:"environments"`
}

type monitorEnvironment struct {
	Status      string     `json:"status"`
	LastCheckIn *time.Time `json:"lastCheckIn"`
	NextCheckIn *time.Time `json:"nextCheckIn"`
}

// toMonitor maps a wire monitor to the exported Monitor. Status/check-in
// times come from the first reported environment when present — jobs here
// only ever check in from one environment — falling back to the monitor's
// own top-level status otherwise.
func (w monitorWire) toMonitor() Monitor {
	m := Monitor{
		Slug:        w.Slug,
		Status:      w.Status,
		LastCheckIn: time.Time{},
		NextCheckIn: time.Time{},
	}

	if len(w.Environments) == 0 {
		return m
	}

	env := w.Environments[0]
	m.Status = env.Status
	if env.LastCheckIn != nil {
		m.LastCheckIn = *env.LastCheckIn
	}
	if env.NextCheckIn != nil {
		m.NextCheckIn = *env.NextCheckIn
	}
	return m
}

// ListMonitors returns the Cron Monitor status of every monitor in the
// configured org.
func (c *client) ListMonitors(ctx context.Context) ([]Monitor, error) {
	cfg, err := c.resolveConfig(ctx)
	if err != nil {
		return nil, err
	}

	if cached, ok := c.cachedMonitors(); ok {
		return cached, nil
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/api/0/organizations/%s/monitors/", baseURL, cfg.Org)

	var wires []monitorWire
	if getErr := c.get(ctx, endpoint, token, &wires); getErr != nil {
		return nil, getErr
	}

	monitors := make([]Monitor, 0, len(wires))
	for _, w := range wires {
		monitors = append(monitors, w.toMonitor())
	}

	c.storeMonitors(monitors)
	return monitors, nil
}

func (c *client) cachedMonitors() ([]Monitor, bool) {
	c.monitorsMu.Lock()
	defer c.monitorsMu.Unlock()
	if c.monitorsCached != nil && time.Since(c.monitorsCachedAt) < cacheTTL {
		return c.monitorsCached, true
	}
	return nil, false
}

func (c *client) storeMonitors(monitors []Monitor) {
	c.monitorsMu.Lock()
	defer c.monitorsMu.Unlock()
	c.monitorsCached = monitors
	c.monitorsCachedAt = time.Now()
}
