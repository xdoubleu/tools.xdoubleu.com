package games

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/games/internal/models"
)

func TestUpperQuartile(t *testing.T) {
	assert.Equal(t, 0, upperQuartile(nil))
	//nolint:exhaustruct // playtime only
	games := []models.Game{{Playtime: 40}, {Playtime: 10}, {Playtime: 30}, {Playtime: 20}}
	assert.Equal(t, 40, upperQuartile(games))
}
