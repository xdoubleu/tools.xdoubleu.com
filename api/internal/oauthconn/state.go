package oauthconn

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"tools.xdoubleu.com/internal/models"
)

const (
	stateTTL   = 10 * time.Minute
	stateBytes = 32
)

type pendingAuth struct {
	provider  models.OAuthProvider
	userID    string
	expiresAt time.Time
}

// StateStore is a single-use, in-memory OAuth CSRF-state map. Single process
// only: it doesn't survive restarts or span replicas.
type StateStore struct {
	mu      sync.Mutex
	pending map[string]pendingAuth
}

func NewStateStore() *StateStore {
	return &StateStore{ //nolint:exhaustruct // mu starts zero-valued
		pending: make(map[string]pendingAuth),
	}
}

// New issues a single-use state tied to userID, so the callback doesn't depend
// on the cookie surviving the external redirect.
func (s *StateStore) New(provider models.OAuthProvider, userID string) string {
	buf := make([]byte, stateBytes)
	_, _ = rand.Read(buf)
	state := hex.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpiredLocked()
	s.pending[state] = pendingAuth{
		provider:  provider,
		userID:    userID,
		expiresAt: time.Now().Add(stateTTL),
	}
	return state
}

// Consume validates and removes state; false if unknown, used or expired.
func (s *StateStore) Consume(
	state string,
) (models.OAuthProvider, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, found := s.pending[state]
	delete(s.pending, state)
	if !found || time.Now().After(entry.expiresAt) {
		return "", "", false
	}
	return entry.provider, entry.userID, true
}

func (s *StateStore) evictExpiredLocked() {
	now := time.Now()
	for k, v := range s.pending {
		if now.After(v.expiresAt) {
			delete(s.pending, k)
		}
	}
}
