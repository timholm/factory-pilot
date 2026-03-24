package act

import (
	"testing"
)

func TestNewClaudeRunner(t *testing.T) {
	runner := NewClaudeRunner("/usr/bin/claude", "/data/repos")
	if runner == nil {
		t.Fatal("NewClaudeRunner returned nil")
	}
	if runner.binary != "/usr/bin/claude" {
		t.Errorf("binary = %q, want /usr/bin/claude", runner.binary)
	}
	if runner.gitDir != "/data/repos" {
		t.Errorf("gitDir = %q, want /data/repos", runner.gitDir)
	}
}

func TestParseFixCommand_WithRepo(t *testing.T) {
	tests := []struct {
		input       string
		wantRepo    string
		wantInstr   string
		shouldError bool
	}{
		{
			input:     "repo:org/myrepo fix the broken test in main_test.go",
			wantRepo:  "org/myrepo",
			wantInstr: "fix the broken test in main_test.go",
		},
		{
			input:     "repo:user/tool update the README with usage instructions",
			wantRepo:  "user/tool",
			wantInstr: "update the README with usage instructions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			repo, instr, err := parseFixCommand(tt.input)
			if tt.shouldError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
			if instr != tt.wantInstr {
				t.Errorf("instruction = %q, want %q", instr, tt.wantInstr)
			}
		})
	}
}

func TestParseFixCommand_WithoutRepo(t *testing.T) {
	input := "fix the flaky integration test"
	repo, instr, err := parseFixCommand(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo != "claude-code-factory" {
		t.Errorf("repo = %q, want 'claude-code-factory' (default)", repo)
	}
	if instr != input {
		t.Errorf("instruction = %q, want %q", instr, input)
	}
}

func TestParseFixCommand_RepoWithoutInstruction(t *testing.T) {
	input := "repo:org/myrepo"
	_, _, err := parseFixCommand(input)
	if err == nil {
		t.Error("parseFixCommand should fail when repo prefix has no instruction")
	}
}

func TestParseFixCommand_EmptyString(t *testing.T) {
	repo, instr, err := parseFixCommand("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty string without repo: prefix gets default repo
	if repo != "claude-code-factory" {
		t.Errorf("repo = %q, want 'claude-code-factory'", repo)
	}
	if instr != "" {
		t.Errorf("instruction = %q, want empty", instr)
	}
}
