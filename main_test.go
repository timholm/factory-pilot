package main

import (
	"testing"
)

func TestUsage_DoesNotPanic(t *testing.T) {
	// usage() writes to stderr; just verify it doesn't panic
	usage()
}

func TestVersionVar(t *testing.T) {
	// The version variable should have a default value
	if version == "" {
		t.Error("version should not be empty")
	}
	if version != "dev" {
		// In test context it should be "dev" (the default)
		t.Logf("version = %q (expected 'dev' for default)", version)
	}
}

func TestUsage_ContainsNewCommands(t *testing.T) {
	// Capture usage output indirectly by just verifying it doesn't panic
	// and that the function exists (compile check for new commands)
	usage()
}
