package pagination_test

import "testing"

// TestMergeGateProbe is the green half of the XW-2 merge-gate probe: with CI
// passing, the required ci-pass check reports success. It is removed before the
// probe PR is closed.
func TestMergeGateProbe(t *testing.T) {
	t.Log("probe: the XW-2 merge gate reports success when checks pass")
}
