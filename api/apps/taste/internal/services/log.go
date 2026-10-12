// Package services builds the taste app's cross-domain consumption log.
package services

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"golang.org/x/sync/errgroup"

	"tools.xdoubleu.com/internal/consumption"
)

// Source is an app exporting its consumed items.
type Source interface {
	GetName() string
	ListConsumption(ctx context.Context, userID string) ([]consumption.Event, error)
}

// LogService merges every source's events.
type LogService struct {
	sources []Source
}

func NewLogService(sources []Source) *LogService {
	return &LogService{sources: sources}
}

// Events returns the user's events from every source whose app name include
// accepts, newest first and undated events last.
func (s *LogService) Events(
	ctx context.Context,
	userID string,
	include func(appName string) bool,
) ([]consumption.Event, error) {
	perSource := make([][]consumption.Event, len(s.sources))
	g, gctx := errgroup.WithContext(ctx)
	for i, src := range s.sources {
		if !include(src.GetName()) {
			continue
		}
		g.Go(func() error {
			events, err := src.ListConsumption(gctx, userID)
			if err != nil {
				return fmt.Errorf("%s: %w", src.GetName(), err)
			}
			perSource[i] = events
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	events := slices.Concat(perSource...)
	SortNewestFirst(events)
	return events, nil
}

// SortNewestFirst orders events by date descending, undated last, then by
// title so the order is stable.
func SortNewestFirst(events []consumption.Event) {
	slices.SortStableFunc(events, func(a, b consumption.Event) int {
		if a.At.IsZero() != b.At.IsZero() {
			if a.At.IsZero() {
				return 1
			}
			return -1
		}
		if c := b.At.Compare(a.At); c != 0 {
			return c
		}
		return cmp.Compare(a.Title, b.Title)
	})
}

// Filter keeps the events of domain; an empty domain keeps all.
func Filter(events []consumption.Event, domain consumption.Domain) []consumption.Event {
	if domain == "" {
		return events
	}
	return slices.DeleteFunc(slices.Clone(events), func(e consumption.Event) bool {
		return e.Domain != domain
	})
}

// IsDomain reports whether d is a known domain.
func IsDomain(d consumption.Domain) bool {
	return slices.Contains(consumption.Domains(), d)
}
