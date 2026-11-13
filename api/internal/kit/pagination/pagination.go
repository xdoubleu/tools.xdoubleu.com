// Package pagination provides shared limit/offset helpers for list RPCs.
package pagination

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// Clamp defaults and caps limit and returns the SQL LIMIT (limit+1, so has_more
// needs no COUNT).
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

// Split trims rows to limit and reports whether more exist.
func Split[T any](rows []T, limit int) ([]T, bool) {
	if len(rows) > limit {
		return rows[:limit], true
	}
	return rows, false
}
