package kobo

import (
	"sync"
	"time"
)

// maxEntriesPerDevice bounds the in-memory ring buffer of captured requests
// per device so debug logging can never grow memory without limit.
const maxEntriesPerDevice = 200

// LogEntry is a single captured device request/response pair.
type LogEntry struct {
	Time         time.Time
	Method       string
	Path         string
	Query        string
	RequestBody  string
	Status       int
	ResponseBody string
	// Notes describes an upstream store merge outcome for diagnostics; never
	// contains credentials.
	Notes string
	// Header maps hold allowlisted sync headers only (lowercase keys); never
	// credentials. UpstreamHeaders is the Kobo store's response.
	RequestHeaders  map[string]string
	ResponseHeaders map[string]string
	UpstreamHeaders map[string]string
}

// LogStore holds, purely in process memory, which Kobo devices have debug
// logging enabled and the recent request/response pairs captured for them.
// Nothing is persisted: the state resets on restart, and disabling logging for
// a device drops its captured entries immediately.
type LogStore struct {
	mu      sync.Mutex
	enabled map[string]bool
	entries map[string][]LogEntry
}

// NewLogStore creates an empty store.
func NewLogStore() *LogStore {
	//nolint:exhaustruct // zero-value mutex is the intended initial state
	return &LogStore{
		enabled: make(map[string]bool),
		entries: make(map[string][]LogEntry),
	}
}

// SetEnabled turns debug logging on or off for a device. Disabling also drops
// any captured entries so the buffered traffic is freed from memory at once.
func (s *LogStore) SetEnabled(deviceID string, on bool) {
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
func (s *LogStore) IsEnabled(deviceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled[deviceID]
}

// Append records a captured entry, evicting the oldest once the per-device cap
// is reached. It is a no-op if logging is not enabled for the device.
func (s *LogStore) Append(deviceID string, entry LogEntry) {
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
func (s *LogStore) List(deviceID string) []LogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.entries[deviceID]
	out := make([]LogEntry, len(src))
	copy(out, src)
	return out
}

// Clear drops the captured entries for a device but leaves logging enabled.
func (s *LogStore) Clear(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, deviceID)
}
