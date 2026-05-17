package dtos_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
)

func TestIsRelativeURL_RootPath(t *testing.T) {
	ok, msg := dtos.IsRelativeURL("/")
	assert.True(t, ok)
	assert.Empty(t, msg)
}

func TestIsRelativeURL_NestedPath(t *testing.T) {
	ok, msg := dtos.IsRelativeURL("/some/path")
	assert.True(t, ok)
	assert.Empty(t, msg)
}

func TestIsRelativeURL_Empty(t *testing.T) {
	ok, msg := dtos.IsRelativeURL("")
	assert.False(t, ok)
	assert.NotEmpty(t, msg)
}

func TestIsRelativeURL_ExternalURL(t *testing.T) {
	ok, msg := dtos.IsRelativeURL("https://example.com")
	assert.False(t, ok)
	assert.NotEmpty(t, msg)
}

func TestIsRelativeURL_DoubleSlash(t *testing.T) {
	// "//evil.com" starts with "/" but second char is also "/" — protocol-relative attack.
	ok, msg := dtos.IsRelativeURL("//evil.com")
	assert.False(t, ok)
	assert.NotEmpty(t, msg)
}

func TestIsRelativeURL_RelativeNoLeadingSlash(t *testing.T) {
	ok, msg := dtos.IsRelativeURL("relative/path")
	assert.False(t, ok)
	assert.NotEmpty(t, msg)
}
