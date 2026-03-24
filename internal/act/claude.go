package act

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ClaudeRunner invokes Claude Code CLI for code and prompt fixes.
type ClaudeRunner struct {
	binary string
	gitDir string
}

// NewClaudeRunner creates a Claude Code runner.
func NewClaudeRunner(binary, gitDir string) *ClaudeRunner {
	return &ClaudeRunner{binary: binary, gitDir: gitDir}
}

// RunCodeFix runs a Claude Code session to fix code in a repo.
// The command format is: "repo:owner/name instruction"
func (c *ClaudeRunner) RunCodeFix(command string) (string, error) {
	repo, instruction, err := parseFixCommand(command)
	if err != nil {
		return "", err
	}

	repoDir := filepath.Join(c.gitDir, repo)
	prompt := fmt.Sprintf("Fix this issue in the codebase. Make the minimal change needed, "+
		"run tests to verify, then commit. Issue: %s", instruction)

	cmd := exec.Command(c.binary,
		"-p", prompt,
		"--max-turns", "5",
		"--output-format", "text",
	)
	cmd.Dir = repoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude code fix: %s: %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

// RunPromptFix edits prompt templates via Claude Code.
func (c *ClaudeRunner) RunPromptFix(command string) (string, error) {
	// Prompt fixes always target claude-code-factory
	repoDir := filepath.Join(c.gitDir, "claude-code-factory")
	prompt := fmt.Sprintf("Improve the prompt templates based on this feedback. "+
		"Edit the relevant prompt files, test them, and commit. Feedback: %s", command)

	cmd := exec.Command(c.binary,
		"-p", prompt,
		"--max-turns", "5",
		"--output-format", "text",
	)
	cmd.Dir = repoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude prompt fix: %s: %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

// parseFixCommand splits "repo:owner/name do the thing" into repo path and instruction.
func parseFixCommand(command string) (string, string, error) {
	if !strings.HasPrefix(command, "repo:") {
		// If no repo prefix, treat entire string as instruction for default repo
		return "claude-code-factory", command, nil
	}

	// Format: "repo:owner/name rest of instruction"
	rest := strings.TrimPrefix(command, "repo:")
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("code fix command must have repo and instruction: %s", command)
	}

	return parts[0], parts[1], nil
}
