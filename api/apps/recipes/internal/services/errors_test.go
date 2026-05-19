package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	iapp "tools.xdoubleu.com/internal/app"
)

func TestRecipesHTTPError_Error(t *testing.T) {
	e := &iapp.HTTPError{Status: 422, Message: "unprocessable entity"}
	assert.Equal(t, "unprocessable entity", e.Error())
}
