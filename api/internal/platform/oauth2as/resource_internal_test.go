package oauth2as

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOwnResourceStrategy(t *testing.T) {
	strategy := ownResourceStrategy("https://tools.xdoubleu.com/api")

	assert.NoError(t, strategy(nil, nil))
	assert.NoError(t, strategy(nil, []string{"https://tools.xdoubleu.com/api/apps/mcp"}))
	assert.NoError(t, strategy(nil, []string{"https://tools.xdoubleu.com/api"}))
	assert.Error(t, strategy(nil, []string{"https://evil.example/api/apps/mcp"}))
	assert.Error(t, strategy(nil, []string{"https://tools.xdoubleu.com/other"}))
	assert.Error(t, strategy(nil, []string{"http://tools.xdoubleu.com/api/apps/mcp"}))
}
