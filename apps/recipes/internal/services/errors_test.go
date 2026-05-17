package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/apps/recipes/internal/services"
)

func TestRecipesHTTPError_Error(t *testing.T) {
	e := &services.HTTPError{Status: 422, Message: "unprocessable entity"}
	assert.Equal(t, "unprocessable entity", e.Error())
}
