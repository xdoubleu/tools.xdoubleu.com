package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestDummyPasswordHash_MatchesStoredCost(t *testing.T) {
	cost, err := bcrypt.Cost(dummyPasswordHash())
	require.NoError(t, err)
	assert.Equal(t, bcryptCost, cost)
}
