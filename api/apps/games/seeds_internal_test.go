package games

import (
	"testing"

	"tools.xdoubleu.com/apps/games/internal/repositories"
	"tools.xdoubleu.com/apps/games/internal/services"
	"tools.xdoubleu.com/apps/games/pkg/steam"
)

// TestSeeds exposes the app's service and repository layers to this
// package's external test files. Defined in a test file, it compiles only
// into the games test binary, so production importers never see the
// internals.
type TestSeeds struct {
	a            *Games
	Services     *services.Services
	Repositories *repositories.Repositories
}

// TestSeeds returns the test-only handle to the app's service layer.
func (a *Games) TestSeeds() *TestSeeds {
	return &TestSeeds{
		a:            a,
		Services:     a.services,
		Repositories: a.repositories,
	}
}

// SetStore swaps the recommendations store for tests, restoring the
// previous store when the test finishes.
func (s *TestSeeds) SetStore(t *testing.T, store steam.StoreSearcher) {
	t.Helper()
	old := s.a.store
	s.a.store = store
	t.Cleanup(func() { s.a.store = old })
}
