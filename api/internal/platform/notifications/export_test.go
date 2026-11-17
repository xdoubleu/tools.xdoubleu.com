package notifications

// NewMemStore exposes the in-memory Store so tests can share one between
// Services, as two processes share a table.
func NewMemStore() Store { return newMemStore() }
