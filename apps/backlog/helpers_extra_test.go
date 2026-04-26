//nolint:testpackage // testing unexported package-level helpers
package backlog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDistributionLabels(t *testing.T) {
	labels := distributionLabels()
	assert.Len(t, labels, 11)
	assert.Equal(t, "0–9%", labels[0])
	assert.Equal(t, "100%", labels[10])
}
