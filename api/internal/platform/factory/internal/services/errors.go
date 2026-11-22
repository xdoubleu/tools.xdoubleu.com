package services

import "errors"

// The kinds of failure a caller maps to its own transport's codes.
var (
	ErrInvalid   = errors.New("invalid request")
	ErrForbidden = errors.New("forbidden")
	ErrConflict  = errors.New("conflict")
)

// domainError is a failure of one kind with a message fit for the user.
type domainError struct {
	kind error
	msg  string
}

func (e *domainError) Error() string { return e.msg }

func (e *domainError) Is(target error) bool { return target == e.kind }
