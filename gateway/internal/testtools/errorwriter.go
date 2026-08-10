// Package testtools contains small fixtures shared by this module's tests.
package testtools

import (
	"errors"
	"net/http"
)

// ErrorWriter wraps an [http.ResponseWriter] and fails every Write, to
// exercise error-handling branches around response writing.
type ErrorWriter struct {
	http.ResponseWriter
}

func (ErrorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
