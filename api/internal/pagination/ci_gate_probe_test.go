package pagination_test

import "testing"

// TestMergeGateProbe deliberately fails to validate that a red PR cannot
// satisfy the branch's required ci-pass check. It is removed before the probe
// PR is closed.
func TestMergeGateProbe(t *testing.T) {
	t.Fatal("deliberately broken test: the XW-2 merge gate must block this PR")
}
