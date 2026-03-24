package act

import (
	"testing"
)

func TestRebuildImage_BadPath(t *testing.T) {
	// RebuildImage should fail when the repo path doesn't exist
	err := RebuildImage("/nonexistent-dir-factory-pilot-test", "test-image")
	if err == nil {
		t.Error("RebuildImage should fail with nonexistent repo path")
	}
}
