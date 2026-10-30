package books

import (
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/internal/services"
)

// TestSeeds exposes the app's service and repository layers to this
// package's external test files. Defined in a test file, it compiles only
// into the books test binary, so production importers never see the
// internals.
type TestSeeds struct {
	Services     *services.Services
	Repositories *repositories.Repositories
}

// TestSeeds returns the test-only handle to the app's service layer.
func (a *Books) TestSeeds() *TestSeeds {
	return &TestSeeds{
		Services:     a.services,
		Repositories: a.repositories,
	}
}
