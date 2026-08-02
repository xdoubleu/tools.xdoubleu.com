// Package pagination provides shared limit/offset helpers for list RPCs.
package pagination

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// Clamp defaults and caps a client-supplied limit, and returns the SQL LIMIT
// to actually request (limit+1, so the caller can detect has_more without a
// separate COUNT query).
func Clamp(limit int32) (int, int) {
	l := int(limit)
	if l <= 0 {
		l = DefaultLimit
	}
	if l > MaxLimit {
		l = MaxLimit
	}
	return l, l + 1
}

// Split trims a limit+1-sized slice down to limit items and reports whether
// more rows exist beyond it.
func Split[T any](rows []T, limit int) ([]T, bool) {
	if len(rows) > limit {
		return rows[:limit], true
	}
	return rows, false
}
