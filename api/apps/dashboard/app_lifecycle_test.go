package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStart(t *testing.T) {
	assert.NoError(t, testApp.Start())
}

func TestApplyMigrations(t *testing.T) {
	assert.NoError(t, testApp.ApplyMigrations(t.Context(), nil))
}
