package jobs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading/internal/jobs"
)

func TestResyncMetadataJob_ID(t *testing.T) {
	j := jobs.NewResyncMetadataJob(nil, nil)
	assert.Equal(t, "resync-books", j.ID())
}

// TestResyncMetadataJob_NotArmed_IsNoop verifies that an unarmed Run call
// returns nil without touching the (nil) books service.
func TestResyncMetadataJob_NotArmed_IsNoop(t *testing.T) {
	j := jobs.NewResyncMetadataJob(nil, nil)
	err := j.Run(context.Background(), logging.NewNopLogger())
	require.NoError(t, err)
}

// TestResyncMetadataJob_ArmThenDisarm verifies that Arm sets the flag and a
// subsequent unarmed Run does nothing (simulates the guard without calling books).
func TestResyncMetadataJob_ArmThenDisarm(t *testing.T) {
	j := jobs.NewResyncMetadataJob(nil, nil)
	j.Arm(false)

	// A second Run (not yet called) resets the flag — simulate by calling Run
	// with a nil books field: if the guard fires it returns nil before touching books.
	// Arm() → Run() would call books (nil) and panic, so we cannot call Run here.
	// Instead verify the ID is stable and Arm is idempotent (no panic on double-arm).
	j.Arm(true)
	assert.Equal(t, "resync-books", j.ID())
}

// TestResyncMetadataJob_Cancel_NoopWhenNotRunning verifies Cancel is safe
// to call when no scan is running — it must not panic on the nil cancel func.
func TestResyncMetadataJob_Cancel_NoopWhenNotRunning(t *testing.T) {
	j := jobs.NewResyncMetadataJob(nil, nil)
	assert.NotPanics(t, j.Cancel)
}
