package diagnose

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/timholm/factory-pilot/internal/analyze"
	"github.com/timholm/factory-pilot/internal/config"
)

// SystemStatus is the full health snapshot of the entire factory pipeline.
type SystemStatus struct {
	Timestamp     time.Time              `json:"timestamp"`
	Papers        PaperStats             `json:"papers"`
	Candidates    CandidateStats         `json:"candidates"`
	BuildQueue    BuildStats             `json:"build_queue"`
	BuildAnalysis *analyze.BuildReport   `json:"build_analysis,omitempty"`
	Pods          []PodStatus            `json:"pods"`
	Router        RouterStats            `json:"router"`
	GitHub        GitHubStats            `json:"github"`
	SpecQuality   SpecQualityStats       `json:"spec_quality"`
	Errors        []string               `json:"errors"`
}

// SpecQualityStats tracks idea-engine spec quality from recent candidates.
type SpecQualityStats struct {
	RecentSpecs    int     `json:"recent_specs"`
	AvgDescLength  float64 `json:"avg_desc_length"`
	WithTechStack  int     `json:"with_tech_stack"`
	WithUseCases   int     `json:"with_use_cases"`
}

// PaperStats tracks research paper ingestion.
type PaperStats struct {
	Total    int `json:"total"`
	Embedded int `json:"embedded"`
	Recent   int `json:"recent_24h"`
}

// CandidateStats tracks idea-engine candidate pipeline.
type CandidateStats struct {
	Total     int            `json:"total"`
	ByStatus  map[string]int `json:"by_status"`
	Approved  int            `json:"approved"`
	Rejected  int            `json:"rejected"`
	Pending   int            `json:"pending"`
	Shipped   int            `json:"shipped"`
}

// BuildStats tracks claude-code-factory build pipeline.
type BuildStats struct {
	Total         int               `json:"total"`
	Shipped       int               `json:"shipped"`
	Failed        int               `json:"failed"`
	Queued        int               `json:"queued"`
	InProgress    int               `json:"in_progress"`
	ErrorPatterns map[string]int    `json:"error_patterns"`
	RecentErrors  []string          `json:"recent_errors"`
}

// PodStatus tracks K8s pod health.
type PodStatus struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Restarts  int    `json:"restarts"`
	Age       string `json:"age"`
	Ready     bool   `json:"ready"`
}

// RouterStats tracks llm-router performance.
type RouterStats struct {
	TotalRequests int            `json:"total_requests"`
	ModelCounts   map[string]int `json:"model_counts"`
	ErrorRate     float64        `json:"error_rate"`
	AvgLatencyMs  float64        `json:"avg_latency_ms"`
}

// GitHubStats tracks published repo metrics.
type GitHubStats struct {
	RepoCount  int `json:"repo_count"`
	TotalStars int `json:"total_stars"`
	TotalForks int `json:"total_forks"`
}

// Collector gathers status from all factory subsystems.
type Collector struct {
	cfg      *config.Config
	pg       *PostgresCollector
	sqlite   *SQLiteCollector
	k8s      *K8sCollector
	gh       *GitHubCollector
	router   *RouterCollector
}

// NewCollector creates a new system status collector.
func NewCollector(cfg *config.Config) *Collector {
	return &Collector{
		cfg:    cfg,
		pg:     NewPostgresCollector(cfg.PostgresURL),
		sqlite: NewSQLiteCollector(cfg.FactoryDataDir),
		k8s:    NewK8sCollector(cfg.K8sNamespace),
		gh:     NewGitHubCollector(cfg.GithubToken, cfg.GithubUser),
		router: NewRouterCollector(),
	}
}

// Collect gathers a full system status snapshot. Runs collectors in parallel
// and captures any errors without failing the entire collection.
func (c *Collector) Collect(ctx context.Context) *SystemStatus {
	status := &SystemStatus{
		Timestamp: time.Now().UTC(),
		Errors:    []string{},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	addError := func(component, msg string) {
		mu.Lock()
		status.Errors = append(status.Errors, fmt.Sprintf("[%s] %s", component, msg))
		mu.Unlock()
	}

	// Postgres: papers + candidates
	wg.Add(1)
	go func() {
		defer wg.Done()
		papers, err := c.pg.CollectPapers(ctx)
		if err != nil {
			addError("postgres/papers", err.Error())
			return
		}
		mu.Lock()
		status.Papers = papers
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		candidates, err := c.pg.CollectCandidates(ctx)
		if err != nil {
			addError("postgres/candidates", err.Error())
			return
		}
		mu.Lock()
		status.Candidates = candidates
		mu.Unlock()
	}()

	// SQLite: build queue
	wg.Add(1)
	go func() {
		defer wg.Done()
		builds, err := c.sqlite.CollectBuilds(ctx)
		if err != nil {
			addError("sqlite/builds", err.Error())
			return
		}
		mu.Lock()
		status.BuildQueue = builds
		mu.Unlock()
	}()

	// K8s: pod status
	wg.Add(1)
	go func() {
		defer wg.Done()
		pods, err := c.k8s.CollectPods(ctx)
		if err != nil {
			addError("k8s/pods", err.Error())
			return
		}
		mu.Lock()
		status.Pods = pods
		mu.Unlock()
	}()

	// GitHub: repo stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		gh, err := c.gh.CollectStats(ctx)
		if err != nil {
			addError("github", err.Error())
			return
		}
		mu.Lock()
		status.GitHub = gh
		mu.Unlock()
	}()

	// Router: request stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		router, err := c.router.CollectStats(ctx)
		if err != nil {
			addError("router", err.Error())
			return
		}
		mu.Lock()
		status.Router = router
		mu.Unlock()
	}()

	// Build analysis: ship rate, failure patterns, language breakdown
	wg.Add(1)
	go func() {
		defer wg.Done()
		dbPath := c.sqlite.dbPath()
		report, err := analyze.AnalyzeBuilds(dbPath)
		if err != nil {
			addError("build_analysis", err.Error())
			return
		}
		mu.Lock()
		status.BuildAnalysis = report
		mu.Unlock()
	}()

	// Spec quality: check recent candidates in Postgres
	wg.Add(1)
	go func() {
		defer wg.Done()
		specQuality, err := c.pg.CollectSpecQuality(ctx)
		if err != nil {
			addError("spec_quality", err.Error())
			return
		}
		mu.Lock()
		status.SpecQuality = specQuality
		mu.Unlock()
	}()

	wg.Wait()
	return status
}

// FormatReport turns a SystemStatus into a human-readable report for the thinker.
func FormatReport(s *SystemStatus) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Factory Status Report — %s\n\n", s.Timestamp.Format(time.RFC3339)))

	b.WriteString("## Research Papers\n")
	b.WriteString(fmt.Sprintf("- Total: %d\n", s.Papers.Total))
	b.WriteString(fmt.Sprintf("- Embedded: %d\n", s.Papers.Embedded))
	b.WriteString(fmt.Sprintf("- Ingested (24h): %d\n\n", s.Papers.Recent))

	b.WriteString("## Idea Candidates\n")
	b.WriteString(fmt.Sprintf("- Total: %d\n", s.Candidates.Total))
	b.WriteString(fmt.Sprintf("- Approved: %d\n", s.Candidates.Approved))
	b.WriteString(fmt.Sprintf("- Pending: %d\n", s.Candidates.Pending))
	b.WriteString(fmt.Sprintf("- Rejected: %d\n", s.Candidates.Rejected))
	b.WriteString(fmt.Sprintf("- Shipped: %d\n", s.Candidates.Shipped))
	if len(s.Candidates.ByStatus) > 0 {
		b.WriteString("- By status:\n")
		for k, v := range s.Candidates.ByStatus {
			b.WriteString(fmt.Sprintf("  - %s: %d\n", k, v))
		}
	}
	b.WriteString("\n")

	b.WriteString("## Build Queue\n")
	b.WriteString(fmt.Sprintf("- Total: %d\n", s.BuildQueue.Total))
	b.WriteString(fmt.Sprintf("- Shipped: %d\n", s.BuildQueue.Shipped))
	b.WriteString(fmt.Sprintf("- Failed: %d\n", s.BuildQueue.Failed))
	b.WriteString(fmt.Sprintf("- Queued: %d\n", s.BuildQueue.Queued))
	b.WriteString(fmt.Sprintf("- In Progress: %d\n", s.BuildQueue.InProgress))
	if len(s.BuildQueue.ErrorPatterns) > 0 {
		b.WriteString("- Error patterns:\n")
		for pattern, count := range s.BuildQueue.ErrorPatterns {
			b.WriteString(fmt.Sprintf("  - %s: %d occurrences\n", pattern, count))
		}
	}
	if len(s.BuildQueue.RecentErrors) > 0 {
		b.WriteString("- Recent errors:\n")
		for _, e := range s.BuildQueue.RecentErrors {
			b.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}
	b.WriteString("\n")

	b.WriteString("## Kubernetes Pods\n")
	if len(s.Pods) == 0 {
		b.WriteString("- No pods found\n")
	}
	for _, p := range s.Pods {
		readyStr := "ready"
		if !p.Ready {
			readyStr = "NOT READY"
		}
		b.WriteString(fmt.Sprintf("- %s: %s (%s), restarts=%d, age=%s\n",
			p.Name, p.Status, readyStr, p.Restarts, p.Age))
	}
	b.WriteString("\n")

	b.WriteString("## LLM Router\n")
	b.WriteString(fmt.Sprintf("- Total requests: %d\n", s.Router.TotalRequests))
	b.WriteString(fmt.Sprintf("- Error rate: %.2f%%\n", s.Router.ErrorRate*100))
	b.WriteString(fmt.Sprintf("- Avg latency: %.0fms\n", s.Router.AvgLatencyMs))
	if len(s.Router.ModelCounts) > 0 {
		b.WriteString("- Model distribution:\n")
		for model, count := range s.Router.ModelCounts {
			b.WriteString(fmt.Sprintf("  - %s: %d\n", model, count))
		}
	}
	b.WriteString("\n")

	if s.BuildAnalysis != nil {
		b.WriteString(s.BuildAnalysis.FormatForThinker())
		b.WriteString("\n")
	}

	b.WriteString("## Spec Quality (Recent Candidates)\n")
	b.WriteString(fmt.Sprintf("- Recent specs: %d\n", s.SpecQuality.RecentSpecs))
	b.WriteString(fmt.Sprintf("- Avg description length: %.0f chars\n", s.SpecQuality.AvgDescLength))
	b.WriteString(fmt.Sprintf("- With tech stack: %d\n", s.SpecQuality.WithTechStack))
	b.WriteString(fmt.Sprintf("- With use cases: %d\n\n", s.SpecQuality.WithUseCases))

	b.WriteString("## GitHub\n")
	b.WriteString(fmt.Sprintf("- Repos: %d\n", s.GitHub.RepoCount))
	b.WriteString(fmt.Sprintf("- Total stars: %d\n", s.GitHub.TotalStars))
	b.WriteString(fmt.Sprintf("- Total forks: %d\n\n", s.GitHub.TotalForks))

	if len(s.Errors) > 0 {
		b.WriteString("## Collection Errors\n")
		for _, e := range s.Errors {
			b.WriteString(fmt.Sprintf("- %s\n", e))
		}
	}

	return b.String()
}
