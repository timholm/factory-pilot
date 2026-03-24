package analyze

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Build represents a single factory build record from SQLite.
type Build struct {
	ID        string
	RepoName  string
	Status    string
	ErrorLog  string
	CreatedAt time.Time
	Duration  time.Duration
	HasTests  bool
	HasReadme bool
	ModPath   string
	Language  string
}

// BuildReport is the output of the build analysis.
type BuildReport struct {
	TotalBuilds   int              `json:"total_builds"`
	ShippedCount  int              `json:"shipped_count"`
	FailedCount   int              `json:"failed_count"`
	ShipRate      float64          `json:"ship_rate"`
	FailureGroups []FailureGroup   `json:"failure_groups"`
	ShippedTraits ShippedTraits    `json:"shipped_traits"`
	LanguageBreak map[string]int   `json:"language_breakdown"`
	RawErrors     []string         `json:"-"`
}

// FailureGroup aggregates a single failure pattern across all failed builds.
type FailureGroup struct {
	Pattern    string   `json:"pattern"`
	Desc       string   `json:"description"`
	Count      int      `json:"count"`
	Percentage float64  `json:"percentage"`
	Examples   []string `json:"examples,omitempty"`
}

// ShippedTraits summarizes what went right in shipped builds.
type ShippedTraits struct {
	WithTests      int     `json:"with_tests"`
	WithReadme     int     `json:"with_readme"`
	CorrectModPath int     `json:"correct_mod_path"`
	TestRate       float64 `json:"test_rate"`
	ReadmeRate     float64 `json:"readme_rate"`
}

// AnalyzeBuilds reads the factory SQLite registry and computes a full build report.
func AnalyzeBuilds(dbPath string) (*BuildReport, error) {
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	defer db.Close()

	builds, err := fetchAllBuilds(db)
	if err != nil {
		return nil, fmt.Errorf("fetch builds: %w", err)
	}

	return AnalyzeBuildList(builds), nil
}

// AnalyzeBuildList processes a list of builds into a BuildReport.
func AnalyzeBuildList(all []Build) *BuildReport {
	report := &BuildReport{
		TotalBuilds:   len(all),
		LanguageBreak: make(map[string]int),
	}

	var shipped, failed []Build
	for _, b := range all {
		switch strings.ToLower(b.Status) {
		case "shipped", "complete", "done":
			shipped = append(shipped, b)
		case "failed", "error":
			failed = append(failed, b)
		}
		if b.Language != "" {
			report.LanguageBreak[b.Language]++
		}
	}

	report.ShippedCount = len(shipped)
	report.FailedCount = len(failed)
	if report.TotalBuilds > 0 {
		report.ShipRate = float64(report.ShippedCount) / float64(report.TotalBuilds)
	}

	// Analyze shipped traits.
	for _, b := range shipped {
		if b.HasTests {
			report.ShippedTraits.WithTests++
		}
		if b.HasReadme {
			report.ShippedTraits.WithReadme++
		}
		if b.ModPath != "" {
			report.ShippedTraits.CorrectModPath++
		}
	}
	if report.ShippedCount > 0 {
		report.ShippedTraits.TestRate = float64(report.ShippedTraits.WithTests) / float64(report.ShippedCount)
		report.ShippedTraits.ReadmeRate = float64(report.ShippedTraits.WithReadme) / float64(report.ShippedCount)
	}

	// Match failure patterns.
	patterns := DefaultPatterns()
	for _, b := range failed {
		report.RawErrors = append(report.RawErrors, b.ErrorLog)
		matched := MatchPatterns(patterns, b.ErrorLog)
		for _, p := range matched {
			p.Matches++
		}
	}

	// Build failure groups sorted by count descending.
	for _, p := range patterns {
		if p.Matches > 0 {
			pct := 0.0
			if report.FailedCount > 0 {
				pct = float64(p.Matches) / float64(report.FailedCount)
			}
			fg := FailureGroup{
				Pattern:    p.Name,
				Desc:       p.Desc,
				Count:      p.Matches,
				Percentage: pct,
			}
			// Collect up to 3 example error snippets for this pattern.
			for _, errLog := range report.RawErrors {
				if p.Regex.MatchString(errLog) && len(fg.Examples) < 3 {
					snippet := errLog
					if len(snippet) > 200 {
						snippet = snippet[:200] + "..."
					}
					fg.Examples = append(fg.Examples, snippet)
				}
			}
			report.FailureGroups = append(report.FailureGroups, fg)
		}
	}

	sort.Slice(report.FailureGroups, func(i, j int) bool {
		return report.FailureGroups[i].Count > report.FailureGroups[j].Count
	})

	return report
}

// String returns a human-readable build report.
func (r *BuildReport) String() string {
	var sb strings.Builder

	sb.WriteString("=== Build Analysis Report ===\n")
	sb.WriteString(fmt.Sprintf("Total builds:  %d\n", r.TotalBuilds))
	sb.WriteString(fmt.Sprintf("Shipped:       %d (%.1f%%)\n", r.ShippedCount, r.ShipRate*100))
	sb.WriteString(fmt.Sprintf("Failed:        %d (%.1f%%)\n", r.FailedCount, (1-r.ShipRate)*100))

	sb.WriteString("\n--- Shipped Build Traits ---\n")
	sb.WriteString(fmt.Sprintf("Has tests:     %d/%d (%.1f%%)\n", r.ShippedTraits.WithTests, r.ShippedCount, r.ShippedTraits.TestRate*100))
	sb.WriteString(fmt.Sprintf("Has README:    %d/%d (%.1f%%)\n", r.ShippedTraits.WithReadme, r.ShippedCount, r.ShippedTraits.ReadmeRate*100))
	sb.WriteString(fmt.Sprintf("Correct mod:   %d/%d\n", r.ShippedTraits.CorrectModPath, r.ShippedCount))

	if len(r.LanguageBreak) > 0 {
		sb.WriteString("\n--- Language Breakdown ---\n")
		for lang, count := range r.LanguageBreak {
			sb.WriteString(fmt.Sprintf("  %-15s %d\n", lang, count))
		}
	}

	sb.WriteString("\n--- Failure Patterns ---\n")
	for _, g := range r.FailureGroups {
		sb.WriteString(fmt.Sprintf("  %-25s %3d  (%.1f%%)  %s\n", g.Pattern, g.Count, g.Percentage*100, g.Desc))
	}

	return sb.String()
}

// JSON returns the report as JSON bytes.
func (r *BuildReport) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// FormatForThinker returns a concise report string suitable for the Opus analysis prompt.
func (r *BuildReport) FormatForThinker() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Build Pipeline Analysis\n"))
	sb.WriteString(fmt.Sprintf("- Ship rate: %.1f%% (%d/%d)\n", r.ShipRate*100, r.ShippedCount, r.TotalBuilds))
	sb.WriteString(fmt.Sprintf("- Failed: %d builds\n", r.FailedCount))

	if report := r.ShippedTraits; r.ShippedCount > 0 {
		sb.WriteString(fmt.Sprintf("- Test coverage rate: %.1f%%\n", report.TestRate*100))
		sb.WriteString(fmt.Sprintf("- README rate: %.1f%%\n", report.ReadmeRate*100))
	}

	if len(r.FailureGroups) > 0 {
		sb.WriteString("\nTop failure patterns:\n")
		limit := 5
		if len(r.FailureGroups) < limit {
			limit = len(r.FailureGroups)
		}
		for _, fg := range r.FailureGroups[:limit] {
			sb.WriteString(fmt.Sprintf("  - %s: %d occurrences (%.1f%%) — %s\n",
				fg.Pattern, fg.Count, fg.Percentage*100, fg.Desc))
		}
	}

	if len(r.LanguageBreak) > 0 {
		sb.WriteString("\nLanguage breakdown:\n")
		for lang, count := range r.LanguageBreak {
			sb.WriteString(fmt.Sprintf("  - %s: %d\n", lang, count))
		}
	}

	return sb.String()
}

// fetchAllBuilds reads all builds from the factory SQLite database.
// Supports both the legacy "builds" table and the "build_queue" table.
func fetchAllBuilds(db *sql.DB) ([]Build, error) {
	// Try the legacy "builds" table first.
	rows, err := db.Query(`
		SELECT id, repo_name, status, COALESCE(error_log, ''),
		       created_at, COALESCE(duration_sec, 0),
		       COALESCE(has_tests, 0), COALESCE(has_readme, 0),
		       COALESCE(mod_path, '')
		FROM builds
		ORDER BY created_at DESC
	`)
	if err == nil {
		defer rows.Close()
		return scanLegacyBuilds(rows)
	}

	// Fall back to "build_queue" table (current factory schema).
	rows2, err2 := db.Query(`
		SELECT CAST(id AS TEXT), name, status, COALESCE(error_log, ''),
		       COALESCE(queued_at, ''), COALESCE(language, '')
		FROM build_queue
		ORDER BY queued_at DESC
	`)
	if err2 != nil {
		return nil, fmt.Errorf("query builds: %w (also tried legacy: %w)", err2, err)
	}
	defer rows2.Close()

	var builds []Build
	for rows2.Next() {
		var b Build
		var queuedAt string
		if err := rows2.Scan(&b.ID, &b.RepoName, &b.Status, &b.ErrorLog,
			&queuedAt, &b.Language); err != nil {
			return nil, fmt.Errorf("scan build_queue: %w", err)
		}
		b.CreatedAt, _ = time.Parse(time.RFC3339, queuedAt)
		if b.CreatedAt.IsZero() {
			b.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", queuedAt)
		}
		b.ModPath = b.Language
		builds = append(builds, b)
	}
	return builds, rows2.Err()
}

func scanLegacyBuilds(rows *sql.Rows) ([]Build, error) {
	var builds []Build
	for rows.Next() {
		var b Build
		var createdAt string
		var durationSec int
		var hasTests, hasReadme int

		if err := rows.Scan(&b.ID, &b.RepoName, &b.Status, &b.ErrorLog,
			&createdAt, &durationSec, &hasTests, &hasReadme, &b.ModPath); err != nil {
			return nil, fmt.Errorf("scan build: %w", err)
		}

		b.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		b.Duration = time.Duration(durationSec) * time.Second
		b.HasTests = hasTests == 1
		b.HasReadme = hasReadme == 1
		builds = append(builds, b)
	}
	return builds, rows.Err()
}
