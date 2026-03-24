package act

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileEdit describes a single file edit operation.
type FileEdit struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// EditRepo clones a repo, applies file edits, verifies the build, and pushes.
func EditRepo(repoURL, branch string, edits []FileEdit) error {
	if len(edits) == 0 {
		return fmt.Errorf("no edits provided")
	}

	// Clone into a temp directory.
	tmpDir, err := os.MkdirTemp("", "factory-pilot-edit-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")

	cloneArgs := []string{"clone", "--depth", "1"}
	if branch != "" {
		cloneArgs = append(cloneArgs, "--branch", branch)
	}
	cloneArgs = append(cloneArgs, repoURL, repoDir)

	if out, err := runGit(tmpDir, cloneArgs...); err != nil {
		return fmt.Errorf("git clone: %s: %w", out, err)
	}

	// Apply edits.
	for _, edit := range edits {
		fullPath := filepath.Join(repoDir, edit.Path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", edit.Path, err)
		}
		if err := os.WriteFile(fullPath, []byte(edit.Content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", edit.Path, err)
		}
	}

	// Verify: go build.
	if out, err := runCmd(repoDir, "go", "build", "./..."); err != nil {
		return fmt.Errorf("go build failed after edits: %s: %w", out, err)
	}

	// Verify: go test.
	if out, err := runCmd(repoDir, "go", "test", "./..."); err != nil {
		return fmt.Errorf("go test failed after edits: %s: %w", out, err)
	}

	// Git add, commit, push.
	if out, err := runGit(repoDir, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %s: %w", out, err)
	}

	var editPaths []string
	for _, e := range edits {
		editPaths = append(editPaths, e.Path)
	}
	commitMsg := fmt.Sprintf("factory-pilot: edit %s", strings.Join(editPaths, ", "))

	if out, err := runGit(repoDir, "commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("git commit: %s: %w", out, err)
	}

	if out, err := runGit(repoDir, "push"); err != nil {
		return fmt.Errorf("git push: %s: %w", out, err)
	}

	return nil
}

// CloneAndFix clones a repo and uses the Claude CLI to fix code based on a prompt.
func CloneAndFix(repoURL string, prompt string, claudeBinary string) error {
	// Clone into a temp directory.
	tmpDir, err := os.MkdirTemp("", "factory-pilot-fix-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")

	if out, err := runGit(tmpDir, "clone", "--depth", "1", repoURL, repoDir); err != nil {
		return fmt.Errorf("git clone: %s: %w", out, err)
	}

	// Run Claude CLI to fix the code.
	fullPrompt := fmt.Sprintf("Fix this codebase based on the following instructions. "+
		"Make the minimal changes needed, ensure go build ./... and go test ./... both pass, "+
		"then commit the changes with a descriptive message. Instructions: %s", prompt)

	cmd := exec.Command(claudeBinary,
		"-p", fullPrompt,
		"--max-turns", "10",
		"--output-format", "text",
	)
	cmd.Dir = repoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude fix: %s: %w", stderr.String(), err)
	}

	// Push the changes Claude made.
	if out, err := runGit(repoDir, "push"); err != nil {
		return fmt.Errorf("git push: %s: %w", out, err)
	}

	return nil
}

// runGit runs a git command in the given directory and returns combined output.
func runGit(dir string, args ...string) (string, error) {
	return runCmd(dir, "git", args...)
}

// runCmd runs a command in the given directory and returns combined output.
func runCmd(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}
