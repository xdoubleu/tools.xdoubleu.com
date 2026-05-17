package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/apps/todos/internal/services"
)

func TestHTTPError_Error(t *testing.T) {
	e := &services.HTTPError{Status: 400, Message: "bad request"}
	assert.Equal(t, "bad request", e.Error())
}
