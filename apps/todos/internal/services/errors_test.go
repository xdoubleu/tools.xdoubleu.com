package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	iapp "tools.xdoubleu.com/internal/app"
)

func TestHTTPError_Error(t *testing.T) {
	e := &iapp.HTTPError{Status: 400, Message: "bad request"}
	assert.Equal(t, "bad request", e.Error())
}
