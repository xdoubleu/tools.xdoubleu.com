package app

import "net/http"

// HTTPError carries an HTTP status alongside a user-facing message.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string { return e.Message }

// NewBadRequestError returns a 400 HTTPError, for services that must not
// import net/http themselves.
func NewBadRequestError(msg string) error {
	return &HTTPError{Status: http.StatusBadRequest, Message: msg}
}

// NewForbiddenError returns a 403 HTTPError, for services that must not
// import net/http themselves.
func NewForbiddenError(msg string) error {
	return &HTTPError{Status: http.StatusForbidden, Message: msg}
}
