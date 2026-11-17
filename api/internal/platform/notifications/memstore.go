package notifications

import (
	"context"
	"sync"
	"time"

	"tools.xdoubleu.com/internal/models"
)

// memStore backs a Service with no database: nothing survives a restart.
type memStore struct {
	mu   sync.Mutex
	next int64
	rows map[int64]*memRow
}

type memRow struct {
	msg    models.OutboxMessage
	due    time.Time
	status string
}

func newMemStore() *memStore {
	//nolint:exhaustruct // mu and next are zero values
	return &memStore{rows: make(map[int64]*memRow)}
}

func (s *memStore) Enqueue(
	_ context.Context,
	to, subject, body string,
) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	m := models.OutboxMessage{
		ID: s.next, To: to, Subject: subject, Body: body, Attempts: 0,
	}
	s.rows[m.ID] = &memRow{msg: m, due: time.Time{}, status: statusPending}
	return m.ID, nil
}

func (s *memStore) ClaimDue(
	_ context.Context,
	now time.Time,
	lease time.Duration,
	limit int,
) ([]models.OutboxMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []models.OutboxMessage
	for id := int64(1); id <= s.next && len(out) < limit; id++ {
		r, ok := s.rows[id]
		if !ok || r.status != statusPending || r.due.After(now) {
			continue
		}
		r.msg.Attempts++
		r.due = now.Add(lease)
		out = append(out, r.msg)
	}
	return out, nil
}

func (s *memStore) MarkSent(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, id)
	return nil
}

func (s *memStore) MarkRetry(
	_ context.Context,
	id int64,
	next time.Time,
	_ string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.rows[id]; ok {
		r.due = next
	}
	return nil
}

func (s *memStore) MarkFailed(_ context.Context, id int64, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, id)
	return nil
}

func (s *memStore) Depth(_ context.Context) (models.OutboxDepth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return models.OutboxDepth{Pending: len(s.rows), Failed: 0}, nil
}

func (s *memStore) PruneFinished(_ context.Context, _ time.Time) error { return nil }
