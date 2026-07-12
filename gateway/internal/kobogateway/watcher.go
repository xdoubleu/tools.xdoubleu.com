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

// DiffKobos compares two FindKobos snapshots (keyed by VolumePath) and
// returns the connect/disconnect events between them. Order is not
// meaningful to callers, so disconnects are reported before connects.
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

// KoboTooltip renders the status-bar tooltip text for the current
// connect/disconnect state.
func KoboTooltip(ev KoboEvent) string {
	if ev.Connected {
		return fmt.Sprintf("Kobo Gateway — Kobo connected (%s)", ev.Kobo.Serial)
	}

	return "Kobo Gateway — no Kobo connected"
}

// KoboMenuLine renders the menu's status line for the current
// connect/disconnect state.
func KoboMenuLine(ev KoboEvent) string {
	if ev.Connected {
		return fmt.Sprintf("Kobo connected: %s", ev.Kobo.Serial)
	}

	return "No Kobo connected"
}

// KoboNotification renders the title/body of the best-effort notification
// posted for the current connect/disconnect state.
func KoboNotification(ev KoboEvent) (title, body string) {
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

	go func() {
		defer close(events)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		prev, _ := FindKobos(volumesRoot)

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
