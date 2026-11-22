package services

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/internal/consumption"
	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/kit/pagination"
)

type hitRates interface {
	HitRates(ctx context.Context, userID string) ([]models.HitRate, error)
}

// ReadService answers what a scope's log and hit rates show.
type ReadService struct {
	log   *LogService
	picks hitRates
	now   func() time.Time
}

func NewReadService(log *LogService, picks hitRates) *ReadService {
	return &ReadService{log: log, picks: picks, now: time.Now}
}

// Stats summarises the scope's log, with the hit rates of the apps it may read.
func (s *ReadService) Stats(
	ctx context.Context,
	sc Scope,
) (Stats, []models.HitRate, error) {
	events, err := s.log.Events(ctx, sc.Subject, sc.Access)
	if err != nil {
		return Stats{}, nil, err
	}
	rates, err := s.picks.HitRates(ctx, sc.Subject)
	if err != nil {
		return Stats{}, nil, err
	}
	visible := make([]models.HitRate, 0, len(rates))
	for _, r := range rates {
		if sc.Access(AppOf(consumption.Domain(r.Domain))) {
			visible = append(visible, r)
		}
	}
	return BuildStats(events, s.now()), visible, nil
}

// Events returns a page of the scope's events of domain (empty for every
// domain), newest first, and whether more follow.
func (s *ReadService) Events(
	ctx context.Context,
	sc Scope,
	domain consumption.Domain,
	limit, offset int32,
) ([]consumption.Event, bool, error) {
	if domain != "" && !IsDomain(domain) {
		return nil, false, app.NewBadRequestError("unknown domain")
	}
	if offset < 0 {
		return nil, false, app.NewBadRequestError("offset must not be negative")
	}
	events, err := s.log.Events(ctx, sc.Subject, sc.Access)
	if err != nil {
		return nil, false, err
	}
	events = Filter(events, domain)

	size, _ := pagination.Clamp(limit)
	start := min(int(offset), len(events))
	end := min(start+size, len(events))
	return events[start:end], end < len(events), nil
}
