package act

import (
	"testing"
	"time"

	"github.com/timholm/factory-pilot/internal/config"
	"github.com/timholm/factory-pilot/internal/think"
)

func TestNewExecutor(t *testing.T) {
	cfg := &config.Config{
		K8sNamespace:   "factory",
		ClaudeBinary:   "claude",
		FactoryGitDir:  "/data/repos",
		FactoryDataDir: "/data/factory",
		MaxFixes:       5,
		DryRun:         true,
	}

	exec := NewExecutor(cfg)
	if exec == nil {
		t.Fatal("NewExecutor returned nil")
	}
	if exec.cfg != cfg {
		t.Error("Executor does not reference the provided config")
	}
	if exec.kubectl == nil {
		t.Error("Executor.kubectl is nil")
	}
	if exec.claude == nil {
		t.Error("Executor.claude is nil")
	}
	if exec.retry == nil {
		t.Error("Executor.retry is nil")
	}
}

func TestExecutorDryRun(t *testing.T) {
	cfg := &config.Config{
		DryRun:         true,
		MaxFixes:       10,
		K8sNamespace:   "factory",
		ClaudeBinary:   "claude",
		FactoryGitDir:  "/tmp/repos",
		FactoryDataDir: "/tmp/factory",
	}

	executor := NewExecutor(cfg)

	issues := []think.Issue{
		{
			Title:       "test issue",
			Severity:    "high",
			RootCause:   "testing",
			FixType:     "kubectl",
			FixCommands: []string{"kubectl rollout restart deployment/test"},
		},
	}

	results := executor.Execute(issues)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Executed {
		t.Error("expected dry-run to not execute")
	}

	if results[0].Output == "" {
		t.Error("expected non-empty dry-run output")
	}
}

func TestExecutorMaxFixes(t *testing.T) {
	cfg := &config.Config{
		DryRun:         true,
		MaxFixes:       2,
		K8sNamespace:   "factory",
		ClaudeBinary:   "claude",
		FactoryGitDir:  "/tmp/repos",
		FactoryDataDir: "/tmp/factory",
	}

	executor := NewExecutor(cfg)

	issues := make([]think.Issue, 5)
	for i := range issues {
		issues[i] = think.Issue{
			Title:       "issue",
			Severity:    "low",
			FixType:     "kubectl",
			FixCommands: []string{"kubectl get pods"},
		}
	}

	results := executor.Execute(issues)

	if len(results) != 2 {
		t.Errorf("expected 2 results (max_fixes=2), got %d", len(results))
	}
}

func TestExecutor_DryRun_AllFixTypes(t *testing.T) {
	cfg := &config.Config{
		DryRun:         true,
		MaxFixes:       10,
		K8sNamespace:   "factory",
		ClaudeBinary:   "claude",
		FactoryGitDir:  "/tmp/repos",
		FactoryDataDir: "/tmp/factory",
	}

	executor := NewExecutor(cfg)

	fixTypes := []string{"kubectl", "code", "prompt", "retry", "config", "docker", "evolve"}
	for _, ft := range fixTypes {
		t.Run(ft, func(t *testing.T) {
			issues := []think.Issue{
				{
					Title:       "test " + ft,
					Severity:    "medium",
					FixType:     ft,
					FixCommands: []string{"some command"},
				},
			}
			results := executor.Execute(issues)
			if len(results) != 1 {
				t.Fatalf("got %d results, want 1", len(results))
			}
			if results[0].Executed {
				t.Error("dry-run should not execute")
			}
			if results[0].Timestamp.IsZero() {
				t.Error("timestamp should not be zero")
			}
			if results[0].Duration == "" {
				t.Error("duration should not be empty")
			}
		})
	}
}

func TestExecutor_Execute_EmptyIssues(t *testing.T) {
	cfg := &config.Config{
		K8sNamespace:   "factory",
		ClaudeBinary:   "claude",
		FactoryGitDir:  "/data/repos",
		FactoryDataDir: "/data/factory",
		MaxFixes:       10,
		DryRun:         true,
	}

	exec := NewExecutor(cfg)
	results := exec.Execute(nil)
	if len(results) != 0 {
		t.Errorf("got %d results for nil issues, want 0", len(results))
	}

	results2 := exec.Execute([]think.Issue{})
	if len(results2) != 0 {
		t.Errorf("got %d results for empty slice, want 0", len(results2))
	}
}

func TestActionResult_Fields(t *testing.T) {
	now := time.Now()
	result := ActionResult{
		Issue: think.Issue{
			Title:    "test issue",
			Severity: "low",
			FixType:  "kubectl",
		},
		Executed:  true,
		Output:    "some output",
		Error:     "",
		Duration:  "1.5s",
		Timestamp: now,
	}

	if !result.Executed {
		t.Error("Executed should be true")
	}
	if result.Output != "some output" {
		t.Errorf("Output = %q, want 'some output'", result.Output)
	}
	if result.Error != "" {
		t.Errorf("Error = %q, want empty", result.Error)
	}
	if result.Duration != "1.5s" {
		t.Errorf("Duration = %q, want '1.5s'", result.Duration)
	}
	if result.Timestamp != now {
		t.Error("Timestamp mismatch")
	}
}

func TestExecutor_DryRun_MultipleCommands(t *testing.T) {
	cfg := &config.Config{
		DryRun:         true,
		MaxFixes:       10,
		K8sNamespace:   "factory",
		ClaudeBinary:   "claude",
		FactoryGitDir:  "/tmp/repos",
		FactoryDataDir: "/tmp/factory",
	}

	executor := NewExecutor(cfg)

	issues := []think.Issue{
		{
			Title:    "multi-command fix",
			Severity: "high",
			FixType:  "kubectl",
			FixCommands: []string{
				"kubectl scale deployment/worker --replicas=0",
				"kubectl apply -f updated.yaml",
				"kubectl scale deployment/worker --replicas=3",
			},
		},
	}

	results := executor.Execute(issues)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Executed {
		t.Error("dry-run should not execute")
	}
	// Dry-run output should mention the commands
	if results[0].Output == "" {
		t.Error("dry-run output should not be empty for multi-command fix")
	}
}
