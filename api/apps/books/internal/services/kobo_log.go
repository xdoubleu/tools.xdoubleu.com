package services

import (
	"sync"
	"time"
)

// maxEntriesPerDevice bounds the in-memory ring buffer of captured requests
// per device so debug logging can never grow memory without limit.
const maxEntriesPerDevice = 200

// KoboLogEntry is a single captured device request/response pair.
type KoboLogEntry struct {
	Time         time.Time
	Method       string
	Path         string
	Query        string
	RequestBody  string
	Status       int
	ResponseBody string
}

// KoboLogStore holds, purely in process memory, which Kobo devices have debug
// logging enabled and the recent request/response pairs captured for them.
// Nothing is persisted: the state resets on restart, and disabling logging for
// a device drops its captured entries immediately.
type KoboLogStore struct {
	mu      sync.Mutex
	enabled map[string]bool
	entries map[string][]KoboLogEntry
}

// NewKoboLogStore creates an empty store.
func NewKoboLogStore() *KoboLogStore {
	//nolint:exhaustruct // zero-value mutex is the intended initial state
	return &KoboLogStore{
		enabled: make(map[string]bool),
		entries: make(map[string][]KoboLogEntry),
	}
}

// SetEnabled turns debug logging on or off for a device. Disabling also drops
// any captured entries so the buffered traffic is freed from memory at once.
func (s *KoboLogStore) SetEnabled(deviceID string, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if on {
		s.enabled[deviceID] = true
		return
	}
	delete(s.enabled, deviceID)
	delete(s.entries, deviceID)
}

// IsEnabled reports whether debug logging is currently on for a device.
func (s *KoboLogStore) IsEnabled(deviceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled[deviceID]
}

// Append records a captured entry, evicting the oldest once the per-device cap
// is reached. It is a no-op if logging is not enabled for the device.
func (s *KoboLogStore) Append(deviceID string, entry KoboLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled[deviceID] {
		return
	}
	s.entries[deviceID] = append(s.entries[deviceID], entry)
	if over := len(s.entries[deviceID]) - maxEntriesPerDevice; over > 0 {
		s.entries[deviceID] = s.entries[deviceID][over:]
	}
}

// List returns a copy of the captured entries for a device, oldest first.
func (s *KoboLogStore) List(deviceID string) []KoboLogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.entries[deviceID]
	out := make([]KoboLogEntry, len(src))
	copy(out, src)
	return out
}

// Clear drops the captured entries for a device but leaves logging enabled.
func (s *KoboLogStore) Clear(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, deviceID)
}
