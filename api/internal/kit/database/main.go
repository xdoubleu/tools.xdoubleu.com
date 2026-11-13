// Package database provides database agnostic helpers.
package database

import "errors"

var (
	// ErrResourceNotFound is returned when a resource doesn't exist.
	ErrResourceNotFound = errors.New("resource not found")
	// ErrResourceConflict is returned on a unique-value conflict.
	ErrResourceConflict = errors.New("resource conflicts with existing resource")
)
