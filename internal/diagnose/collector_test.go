package diagnose

import (
	"strings"
	"testing"
	"time"

	"github.com/timholm/factory-pilot/internal/config"
)

func TestNewCollector(t *testing.T) {
	cfg := &config.Config{
		PostgresURL:    "postgres://localhost:5432/test",
		FactoryDataDir: "/data/factory",
		K8sNamespace:   "factory",
		GithubToken:    "ghp_test",
		GithubUser:     "testuser",
	}

	collector := NewCollector(cfg)
	if collector == nil {
		t.Fatal("NewCollector returned nil")
	}
	if collector.cfg != cfg {
		t.Error("Collector does not reference the provided config")
	}
	if collector.pg == nil {
		t.Error("Collector.pg is nil")
	}
	if collector.sqlite == nil {
		t.Error("Collector.sqlite is nil")
	}
	if collector.k8s == nil {
		t.Error("Collector.k8s is nil")
	}
	if collector.gh == nil {
		t.Error("Collector.gh is nil")
	}
	if collector.router == nil {
		t.Error("Collector.router is nil")
	}
}

func TestFormatReport(t *testing.T) {
	status := &SystemStatus{
		Timestamp: time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC),
		Papers: PaperStats{
			Total:    1500,
			Embedded: 1200,
			Recent:   45,
		},
		Candidates: CandidateStats{
			Total:    200,
			ByStatus: map[string]int{"approved": 50, "pending": 30, "rejected": 100, "shipped": 20},
			Approved: 50,
			Rejected: 100,
			Pending:  30,
			Shipped:  20,
		},
		BuildQueue: BuildStats{
			Total:         80,
			Shipped:       60,
			Failed:        10,
			Queued:        5,
			InProgress:    5,
			ErrorPatterns: map[string]int{"compilation_error": 5, "timeout": 3},
			RecentErrors:  []string{"build failed: compile error in main.go", "timeout after 300s"},
		},
		Pods: []PodStatus{
			{Name: "factory-builder-0", Namespace: "factory", Status: "Running", Restarts: 0, Age: "2d", Ready: true},
			{Name: "idea-engine-0", Namespace: "factory", Status: "CrashLoopBackOff", Restarts: 15, Age: "1d", Ready: false},
		},
		Router: RouterStats{
			TotalRequests: 5000,
			ModelCounts:   map[string]int{"claude-sonnet-4": 3000, "claude-haiku": 2000},
			ErrorRate:     0.02,
			AvgLatencyMs:  450,
		},
		GitHub: GitHubStats{
			RepoCount:  25,
			TotalStars: 150,
			TotalForks: 30,
		},
		Errors: []string{"[router] connection refused"},
	}

	report := FormatReport(status)

	// Check key sections exist
	checks := []string{
		"Factory Status Report",
		"Research Papers",
		"Total: 1500",
		"Embedded: 1200",
		"Idea Candidates",
		"Approved: 50",
		"Build Queue",
		"Shipped: 60",
		"Failed: 10",
		"compilation_error",
		"Kubernetes Pods",
		"factory-builder-0",
		"CrashLoopBackOff",
		"NOT READY",
		"restarts=15",
		"LLM Router",
		"Total requests: 5000",
		"GitHub",
		"Repos: 25",
		"Collection Errors",
		"[router] connection refused",
	}

	for _, check := range checks {
		if !strings.Contains(report, check) {
			t.Errorf("report missing expected content: %q", check)
		}
	}
}

func TestFormatReport_Empty(t *testing.T) {
	status := &SystemStatus{
		Timestamp: time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
		Errors:    []string{},
	}

	report := FormatReport(status)

	if !strings.Contains(report, "Factory Status Report") {
		t.Error("report should contain title")
	}
	if !strings.Contains(report, "2025-01-15") {
		t.Error("report should contain formatted date")
	}
	if !strings.Contains(report, "No pods found") {
		t.Error("report should say no pods found when pod list is empty")
	}
	if strings.Contains(report, "Collection Errors") {
		t.Error("report should not contain errors section when no errors")
	}
}

func TestFormatReport_ZeroValues(t *testing.T) {
	status := &SystemStatus{
		Timestamp: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		Papers:    PaperStats{Total: 0, Embedded: 0, Recent: 0},
		Candidates: CandidateStats{
			Total:    0,
			ByStatus: map[string]int{},
		},
		BuildQueue: BuildStats{
			ErrorPatterns: map[string]int{},
		},
		Router: RouterStats{
			ModelCounts: map[string]int{},
		},
		Errors: []string{},
	}

	report := FormatReport(status)
	if !strings.Contains(report, "Total: 0") {
		t.Error("report should show zero totals")
	}
}

func TestFormatReport_MultipleErrors(t *testing.T) {
	status := &SystemStatus{
		Timestamp: time.Now().UTC(),
		Errors: []string{
			"[postgres] conn refused",
			"[k8s] timeout",
			"[router] 503",
		},
	}

	report := FormatReport(status)
	if !strings.Contains(report, "Collection Errors") {
		t.Error("should have errors section")
	}
	if !strings.Contains(report, "[postgres] conn refused") {
		t.Error("should contain postgres error")
	}
	if !strings.Contains(report, "[k8s] timeout") {
		t.Error("should contain k8s error")
	}
	if !strings.Contains(report, "[router] 503") {
		t.Error("should contain router error")
	}
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"build failed: compile error", "compilation_error"},
		{"Build Error: undefined variable", "compilation_error"},
		{"test suite failed", "test_failure"},
		{"unit tests failed: 3 assertions", "test_failure"},
		{"operation timed out after 300s", "timeout"},
		{"request timeout", "timeout"},
		{"rate limit exceeded (429)", "rate_limit"},
		{"HTTP 429 Too Many Requests", "rate_limit"},
		{"permission denied", "auth_error"},
		{"authentication failed", "auth_error"},
		{"resource not found (404)", "not_found"},
		{"endpoint returned 404", "not_found"},
		{"prompt token limit exceeded", "prompt_error"},
		{"token count too high", "prompt_error"},
		{"something completely different", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := categorizeError(tt.input)
			if got != tt.expected {
				t.Errorf("categorizeError(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSystemStatus_Types(t *testing.T) {
	status := SystemStatus{
		Timestamp: time.Now(),
		Papers:    PaperStats{Total: 1, Embedded: 1, Recent: 1},
		Candidates: CandidateStats{
			Total:    5,
			ByStatus: map[string]int{"approved": 2, "pending": 3},
			Approved: 2,
			Pending:  3,
		},
		BuildQueue: BuildStats{
			Total:         10,
			Shipped:       5,
			Failed:        2,
			Queued:        2,
			InProgress:    1,
			ErrorPatterns: map[string]int{"timeout": 1},
			RecentErrors:  []string{"err1"},
		},
		Pods: []PodStatus{
			{Name: "pod1", Namespace: "ns", Status: "Running", Restarts: 0, Age: "1h", Ready: true},
		},
		Router: RouterStats{
			TotalRequests: 100,
			ModelCounts:   map[string]int{"opus": 50},
			ErrorRate:     0.01,
			AvgLatencyMs:  100.0,
		},
		GitHub: GitHubStats{RepoCount: 10, TotalStars: 50, TotalForks: 10},
		Errors: []string{"test"},
	}

	if status.Papers.Total != 1 {
		t.Error("PaperStats.Total mismatch")
	}
	if status.Candidates.ByStatus["approved"] != 2 {
		t.Error("CandidateStats.ByStatus mismatch")
	}
	if status.BuildQueue.ErrorPatterns["timeout"] != 1 {
		t.Error("BuildStats.ErrorPatterns mismatch")
	}
	if len(status.Pods) != 1 {
		t.Error("Pods length mismatch")
	}
	if status.Router.ModelCounts["opus"] != 50 {
		t.Error("RouterStats.ModelCounts mismatch")
	}
	if status.GitHub.RepoCount != 10 {
		t.Error("GitHubStats.RepoCount mismatch")
	}
}

func TestNewPostgresCollector(t *testing.T) {
	pg := NewPostgresCollector("postgres://localhost:5432/test")
	if pg == nil {
		t.Fatal("NewPostgresCollector returned nil")
	}
	if pg.connStr != "postgres://localhost:5432/test" {
		t.Errorf("connStr = %q, want postgres://localhost:5432/test", pg.connStr)
	}
}

func TestNewSQLiteCollector(t *testing.T) {
	s := NewSQLiteCollector("/data/factory")
	if s == nil {
		t.Fatal("NewSQLiteCollector returned nil")
	}
	if s.dataDir != "/data/factory" {
		t.Errorf("dataDir = %q, want /data/factory", s.dataDir)
	}
	if s.dbPath() != "/data/factory/factory.db" {
		t.Errorf("dbPath = %q, want /data/factory/factory.db", s.dbPath())
	}
}

func TestNewK8sCollector(t *testing.T) {
	k := NewK8sCollector("staging")
	if k == nil {
		t.Fatal("NewK8sCollector returned nil")
	}
	if k.namespace != "staging" {
		t.Errorf("namespace = %q, want staging", k.namespace)
	}
}

func TestNewGitHubCollector(t *testing.T) {
	gh := NewGitHubCollector("token123", "user")
	if gh == nil {
		t.Fatal("NewGitHubCollector returned nil")
	}
	if gh.token != "token123" {
		t.Errorf("token = %q, want token123", gh.token)
	}
	if gh.user != "user" {
		t.Errorf("user = %q, want user", gh.user)
	}
}

func TestNewRouterCollector(t *testing.T) {
	r := NewRouterCollector()
	if r == nil {
		t.Fatal("NewRouterCollector returned nil")
	}
	if r.baseURL == "" {
		t.Error("baseURL should not be empty")
	}
	if r.client == nil {
		t.Error("client should not be nil")
	}
}

func TestPodStatus_Fields(t *testing.T) {
	pod := PodStatus{
		Name:      "worker-abc",
		Namespace: "factory",
		Status:    "Running",
		Restarts:  3,
		Age:       "2h",
		Ready:     true,
	}

	if pod.Name != "worker-abc" {
		t.Errorf("Name = %q", pod.Name)
	}
	if pod.Namespace != "factory" {
		t.Errorf("Namespace = %q", pod.Namespace)
	}
	if pod.Status != "Running" {
		t.Errorf("Status = %q", pod.Status)
	}
	if pod.Restarts != 3 {
		t.Errorf("Restarts = %d", pod.Restarts)
	}
	if !pod.Ready {
		t.Error("Ready should be true")
	}
}
