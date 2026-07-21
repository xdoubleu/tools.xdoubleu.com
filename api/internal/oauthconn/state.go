package oauthconn

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"tools.xdoubleu.com/internal/models"
)

const (
	// stateTTL is how long an issued OAuth CSRF state stays valid before it
	// must be re-issued by revisiting the connect flow.
	stateTTL = 10 * time.Minute
	// stateBytes is the size of the random token backing each state value.
	stateBytes = 32
)

type pendingAuth struct {
	provider  models.OAuthProvider
	userID    string
	expiresAt time.Time
}

// StateStore is a single-use, in-memory CSRF-state map for the OAuth
// initiate/callback redirect leg.
//
// ponytail: single-process, single-replica assumption (matches other
// in-memory state in this codebase, e.g. the kobo-gateway log store) — move
// to a DB table if this ever needs to survive a restart or run behind more
// than one replica.
type StateStore struct {
	mu      sync.Mutex
	pending map[string]pendingAuth
}

func NewStateStore() *StateStore {
	return &StateStore{ //nolint:exhaustruct // mu starts zero-valued
		pending: make(map[string]pendingAuth),
	}
}

// New issues a fresh single-use state token for provider, tying it to userID
// so the callback leg trusts who initiated the connection regardless of
// whether the browser's cookie survives the external redirect.
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

// Consume validates and removes state, returning the provider/user it was
// issued for. The final bool is false if state is unknown, already used, or
// expired.
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
