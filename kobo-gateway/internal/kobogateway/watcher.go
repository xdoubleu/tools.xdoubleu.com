package kobogateway

import (
	"context"
	"fmt"
	"time"
)

// KoboEvent reports a Kobo appearing or disappearing under the watched
// volumes root.
type KoboEvent struct {
	Connected bool
	Kobo      Kobo
}

// DiffKobos returns the connect/disconnect events between two FindKobos
// snapshots (keyed by VolumePath), disconnects first.
func DiffKobos(prev, curr []Kobo) []KoboEvent {
	prevByPath := make(map[string]Kobo, len(prev))
	for _, k := range prev {
		prevByPath[k.VolumePath] = k
	}

	currByPath := make(map[string]Kobo, len(curr))
	for _, k := range curr {
		currByPath[k.VolumePath] = k
	}

	events := make([]KoboEvent, 0)

	for _, k := range prev {
		if _, stillThere := currByPath[k.VolumePath]; !stillThere {
			events = append(events, KoboEvent{Connected: false, Kobo: k})
		}
	}

	for _, k := range curr {
		if _, wasThere := prevByPath[k.VolumePath]; !wasThere {
			events = append(events, KoboEvent{Connected: true, Kobo: k})
		}
	}

	return events
}

// ShortReleaseLen matches web/components/Footer.tsx's truncation.
const ShortReleaseLen = 7

// ShortRelease truncates a SHA for display; short values pass through.
func ShortRelease(release string) string {
	if len(release) <= ShortReleaseLen {
		return release
	}

	return release[:ShortReleaseLen]
}

// KoboTooltip renders the status-bar tooltip, prefixed with the short release.
func KoboTooltip(ev KoboEvent, release string) string {
	release = ShortRelease(release)

	if ev.Connected {
		return fmt.Sprintf("Kobo Gateway %s — Kobo connected (%s)", release, ev.Kobo.Serial)
	}

	return fmt.Sprintf("Kobo Gateway %s — no Kobo connected", release)
}

// KoboMenuLine renders the menu's status line for the current
// connect/disconnect state.
func KoboMenuLine(ev KoboEvent) string {
	if ev.Connected {
		return fmt.Sprintf("Kobo connected: %s", ev.Kobo.Serial)
	}

	return "No Kobo connected"
}

// KoboNotification renders the (title, body) of the best-effort
// notification posted for the current connect/disconnect state.
func KoboNotification(ev KoboEvent) (string, string) {
	if ev.Connected {
		return "Kobo connected", "Serial " + ev.Kobo.Serial
	}

	return "Kobo disconnected", ""
}

// Watch polls volumesRoot every interval and sends a KoboEvent each time a
// Kobo connects or disconnects. It closes the returned channel once ctx is
// done.
//
// ponytail: poll loop over FindKobos; swap for a DiskArbitration mount
// observer if the interval's latency or battery cost ever matters.
func Watch(
	ctx context.Context,
	volumesRoot string,
	interval time.Duration,
) <-chan KoboEvent {
	events := make(chan KoboEvent)

	// Snapshot synchronously so callers get a deterministic starting point.
	prev, _ := FindKobos(volumesRoot)

	go func() {
		defer close(events)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				curr, err := FindKobos(volumesRoot)
				if err != nil {
					continue
				}

				for _, ev := range DiffKobos(prev, curr) {
					select {
					case events <- ev:
					case <-ctx.Done():
						return
					}
				}

				prev = curr
			}
		}
	}()

	return events
}
