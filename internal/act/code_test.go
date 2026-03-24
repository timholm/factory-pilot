package act

import (
	"testing"
)

func TestEditRepo_NoEdits(t *testing.T) {
	err := EditRepo("https://github.com/test/repo", "main", nil)
	if err == nil {
		t.Error("EditRepo should fail with no edits")
	}
}

func TestEditRepo_EmptyEdits(t *testing.T) {
	err := EditRepo("https://github.com/test/repo", "main", []FileEdit{})
	if err == nil {
		t.Error("EditRepo should fail with empty edits")
	}
}

func TestFileEdit_Fields(t *testing.T) {
	edit := FileEdit{
		Path:    "main.go",
		Content: "package main\n",
	}
	if edit.Path != "main.go" {
		t.Errorf("Path = %q, want main.go", edit.Path)
	}
	if edit.Content != "package main\n" {
		t.Errorf("Content = %q", edit.Content)
	}
}

func TestRunCmd_BadCommand(t *testing.T) {
	_, err := runCmd("/tmp", "nonexistent-binary-factory-pilot-test")
	if err == nil {
		t.Error("runCmd should fail with nonexistent binary")
	}
}

func TestRunGit_BadDir(t *testing.T) {
	_, err := runGit("/nonexistent-dir-factory-pilot-test", "status")
	if err == nil {
		t.Error("runGit should fail with nonexistent dir")
	}
}
