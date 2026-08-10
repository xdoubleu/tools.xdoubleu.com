package errortools_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/errortools"
)

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
