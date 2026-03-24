package act

import (
	"testing"
)

func TestNewRetryRunner(t *testing.T) {
	runner := NewRetryRunner("/data/factory")
	if runner == nil {
		t.Fatal("NewRetryRunner returned nil")
	}
	if runner.dataDir != "/data/factory" {
		t.Errorf("dataDir = %q, want /data/factory", runner.dataDir)
	}
}

func TestRetryRunner_Run_TooShort(t *testing.T) {
	runner := NewRetryRunner("/tmp")
	_, err := runner.Run("retry")
	if err == nil {
		t.Error("Run should fail on too-short command")
	}
}

func TestRetryRunner_Run_UnknownSubcommand(t *testing.T) {
	runner := NewRetryRunner("/tmp")
	_, err := runner.Run("retry unknown-thing")
	if err == nil {
		t.Error("Run should fail on unknown subcommand")
	}
}

func TestRetryRunner_Run_BuildWithoutID(t *testing.T) {
	runner := NewRetryRunner("/tmp")
	_, err := runner.Run("retry build")
	if err == nil {
		t.Error("Run should fail when build has no ID")
	}
}

func TestRetryRunner_Run_DBNotFound(t *testing.T) {
	// Point at a directory that doesn't contain factory.db
	runner := NewRetryRunner("/tmp/nonexistent-dir-for-testing")

	// All commands should fail because the DB doesn't exist
	_, err := runner.Run("retry all-failed")
	if err == nil {
		t.Error("Run should fail when DB path doesn't exist")
	}

	_, err = runner.Run("retry build 123")
	if err == nil {
		t.Error("Run should fail when DB path doesn't exist")
	}

	_, err = runner.Run("retry recent 5")
	if err == nil {
		t.Error("Run should fail when DB path doesn't exist")
	}
}
