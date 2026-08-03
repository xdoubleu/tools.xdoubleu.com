//nolint:testpackage // testing unexported store internals (ring-buffer cap)
package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logEntry builds a KoboLogEntry for tests without tripping exhaustruct on
// every call site.
func logEntry(method, path string) KoboLogEntry {
	return KoboLogEntry{
		Time:         time.Now(),
		Method:       method,
		Path:         path,
		Query:        "",
		RequestBody:  "",
		Status:       0,
		ResponseBody: "",
	}
}

func TestKoboLogStore_DisabledByDefault(t *testing.T) {
	s := NewKoboLogStore()
	assert.False(t, s.IsEnabled("dev-1"))
	// Append is a no-op while disabled.
	s.Append("dev-1", logEntry("GET", ""))
	assert.Empty(t, s.List("dev-1"))
}

func TestKoboLogStore_EnableAppendList(t *testing.T) {
	s := NewKoboLogStore()
	s.SetEnabled("dev-1", true)
	require.True(t, s.IsEnabled("dev-1"))

	s.Append("dev-1", logEntry("GET", "/a"))
	s.Append("dev-1", logEntry("PUT", "/b"))

	got := s.List("dev-1")
	require.Len(t, got, 2)
	assert.Equal(t, "/a", got[0].Path)
	assert.Equal(t, "/b", got[1].Path)
}

func TestKoboLogStore_DisableDropsEntries(t *testing.T) {
	s := NewKoboLogStore()
	s.SetEnabled("dev-1", true)
	s.Append("dev-1", logEntry("GET", ""))
	require.Len(t, s.List("dev-1"), 1)

	// Disabling must free the buffered traffic immediately.
	s.SetEnabled("dev-1", false)
	assert.False(t, s.IsEnabled("dev-1"))
	assert.Empty(t, s.List("dev-1"))
}

func TestKoboLogStore_ClearKeepsEnabled(t *testing.T) {
	s := NewKoboLogStore()
	s.SetEnabled("dev-1", true)
	s.Append("dev-1", logEntry("GET", ""))

	s.Clear("dev-1")
	assert.Empty(t, s.List("dev-1"))
	assert.True(t, s.IsEnabled("dev-1"), "Clear must not disable logging")

	// Still capturing after a clear.
	s.Append("dev-1", logEntry("POST", ""))
	assert.Len(t, s.List("dev-1"), 1)
}

func TestKoboLogStore_RingBufferCap(t *testing.T) {
	s := NewKoboLogStore()
	s.SetEnabled("dev-1", true)
	for i := 0; i < maxEntriesPerDevice+50; i++ {
		s.Append("dev-1", logEntry("GET", "/x"))
	}
	assert.Len(t, s.List("dev-1"), maxEntriesPerDevice)
}

func TestKoboLogStore_ListReturnsCopy(t *testing.T) {
	s := NewKoboLogStore()
	s.SetEnabled("dev-1", true)
	s.Append("dev-1", logEntry("GET", "/orig"))

	got := s.List("dev-1")
	got[0].Path = "/mutated"

	assert.Equal(t, "/orig", s.List("dev-1")[0].Path,
		"mutating the returned slice must not affect the store")
}

func TestKoboLogStore_PerDeviceIsolation(t *testing.T) {
	s := NewKoboLogStore()
	s.SetEnabled("dev-1", true)
	s.SetEnabled("dev-2", true)
	s.Append("dev-1", logEntry("GET", "/one"))

	assert.Len(t, s.List("dev-1"), 1)
	assert.Empty(t, s.List("dev-2"))
}
