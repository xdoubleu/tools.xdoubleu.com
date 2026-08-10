package services_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/games/internal/services"
)

func TestDistributionLabels(t *testing.T) {
	labels := services.DistributionLabels()
	assert.Len(t, labels, 11)
	assert.Equal(t, "0–9%", labels[0])
	assert.Equal(t, "100%", labels[10])
}

func TestGetDistributionBucket_InvalidBucket(t *testing.T) {
	s := services.NewProgressService(nil, nil)

	_, _, err := s.GetDistributionBucket(context.Background(), "user-1", -1)
	assert.ErrorIs(t, err, services.ErrInvalidBucket)

	_, _, err = s.GetDistributionBucket(context.Background(), "user-1", 11)
	assert.ErrorIs(t, err, services.ErrInvalidBucket)
}
