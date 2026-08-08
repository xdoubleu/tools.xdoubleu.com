package errortools_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/errortools"
)

func TestNotFoundError(t *testing.T) {
	t.Parallel()

	err := errortools.NewNotFoundError("book", "abc", "id")
	assert.Equal(t, "book with id 'abc' doesn't exist", err.Error())
	assert.Equal(t, "id", err.JSONField)
}

func TestConflictError(t *testing.T) {
	t.Parallel()

	err := errortools.NewConflictError("book", 1, "id")
	assert.Equal(t, "book with id '1' already exists", err.Error())
}

func TestBadRequestError(t *testing.T) {
	t.Parallel()

	err := errortools.NewBadRequestError(errors.New("bad input"))
	assert.Equal(t, "bad input", err.Error())
}

func TestUnauthorizedError(t *testing.T) {
	t.Parallel()

	err := errortools.NewUnauthorizedError(errors.New("no token"))
	assert.Equal(t, "no token", err.Error())
}
