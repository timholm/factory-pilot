package analyze

import (
	"strings"
	"testing"
)

func TestAnalyzeBuildListEmpty(t *testing.T) {
	report := AnalyzeBuildList(nil)

	if report.TotalBuilds != 0 {
		t.Errorf("TotalBuilds = %d, want 0", report.TotalBuilds)
	}
	if report.ShippedCount != 0 {
		t.Errorf("ShippedCount = %d, want 0", report.ShippedCount)
	}
	if report.FailedCount != 0 {
		t.Errorf("FailedCount = %d, want 0", report.FailedCount)
	}
	if report.ShipRate != 0 {
		t.Errorf("ShipRate = %f, want 0", report.ShipRate)
	}
}

func TestAnalyzeBuildListAllShipped(t *testing.T) {
	builds := []Build{
		{ID: "1", Status: "shipped", HasTests: true, HasReadme: true, ModPath: "github.com/test/a"},
		{ID: "2", Status: "shipped", HasTests: true, HasReadme: false, ModPath: "github.com/test/b"},
		{ID: "3", Status: "shipped", HasTests: false, HasReadme: true, ModPath: ""},
	}

	report := AnalyzeBuildList(builds)

	if report.TotalBuilds != 3 {
		t.Errorf("TotalBuilds = %d, want 3", report.TotalBuilds)
	}
	if report.ShippedCount != 3 {
		t.Errorf("ShippedCount = %d, want 3", report.ShippedCount)
	}
	if report.FailedCount != 0 {
		t.Errorf("FailedCount = %d, want 0", report.FailedCount)
	}
	if report.ShipRate != 1.0 {
		t.Errorf("ShipRate = %f, want 1.0", report.ShipRate)
	}
	if report.ShippedTraits.WithTests != 2 {
		t.Errorf("WithTests = %d, want 2", report.ShippedTraits.WithTests)
	}
	if report.ShippedTraits.WithReadme != 2 {
		t.Errorf("WithReadme = %d, want 2", report.ShippedTraits.WithReadme)
	}
	if report.ShippedTraits.CorrectModPath != 2 {
		t.Errorf("CorrectModPath = %d, want 2", report.ShippedTraits.CorrectModPath)
	}
}

func TestAnalyzeBuildListWithFailures(t *testing.T) {
	builds := []Build{
		{ID: "1", Status: "shipped", HasTests: true, HasReadme: true, ModPath: "github.com/test/a"},
		{ID: "2", Status: "failed", ErrorLog: "no test files found in project"},
		{ID: "3", Status: "failed", ErrorLog: "build failed: syntax error at line 10"},
		{ID: "4", Status: "failed", ErrorLog: "no test files found; build was incomplete"},
	}

	report := AnalyzeBuildList(builds)

	if report.TotalBuilds != 4 {
		t.Errorf("TotalBuilds = %d, want 4", report.TotalBuilds)
	}
	if report.ShippedCount != 1 {
		t.Errorf("ShippedCount = %d, want 1", report.ShippedCount)
	}
	if report.FailedCount != 3 {
		t.Errorf("FailedCount = %d, want 3", report.FailedCount)
	}

	if len(report.FailureGroups) == 0 {
		t.Fatal("expected failure groups, got none")
	}

	found := false
	for _, fg := range report.FailureGroups {
		if fg.Pattern == "no_test_files" {
			found = true
			if fg.Count != 2 {
				t.Errorf("no_test_files count = %d, want 2", fg.Count)
			}
		}
	}
	if !found {
		t.Error("expected no_test_files pattern in failure groups")
	}

	// Verify sorted by count descending.
	for i := 1; i < len(report.FailureGroups); i++ {
		if report.FailureGroups[i].Count > report.FailureGroups[i-1].Count {
			t.Error("failure groups not sorted by count descending")
		}
	}
}

func TestAnalyzeBuildListLanguageBreakdown(t *testing.T) {
	builds := []Build{
		{ID: "1", Status: "shipped", Language: "go"},
		{ID: "2", Status: "shipped", Language: "go"},
		{ID: "3", Status: "shipped", Language: "rust"},
		{ID: "4", Status: "failed", Language: "go"},
	}

	report := AnalyzeBuildList(builds)

	if report.LanguageBreak["go"] != 3 {
		t.Errorf("go count = %d, want 3", report.LanguageBreak["go"])
	}
	if report.LanguageBreak["rust"] != 1 {
		t.Errorf("rust count = %d, want 1", report.LanguageBreak["rust"])
	}
}

func TestBuildReportString(t *testing.T) {
	report := &BuildReport{
		TotalBuilds:  10,
		ShippedCount: 7,
		FailedCount:  3,
		ShipRate:     0.7,
		LanguageBreak: map[string]int{"go": 8, "rust": 2},
		ShippedTraits: ShippedTraits{
			WithTests:      6,
			WithReadme:     5,
			CorrectModPath: 7,
			TestRate:       0.857,
			ReadmeRate:     0.714,
		},
		FailureGroups: []FailureGroup{
			{Pattern: "compilation_error", Desc: "Code failed to compile", Count: 2, Percentage: 0.667},
		},
	}

	s := report.String()
	if !strings.Contains(s, "Build Analysis Report") {
		t.Error("report string missing header")
	}
	if !strings.Contains(s, "Total builds:  10") {
		t.Error("report string missing total builds")
	}
	if !strings.Contains(s, "compilation_error") {
		t.Error("report string missing failure pattern")
	}
	if !strings.Contains(s, "Language Breakdown") {
		t.Error("report string missing language breakdown")
	}
}

func TestBuildReportJSON(t *testing.T) {
	report := &BuildReport{
		TotalBuilds:   5,
		ShippedCount:  3,
		FailedCount:   2,
		ShipRate:      0.6,
		LanguageBreak: map[string]int{},
	}

	data, err := report.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"total_builds": 5`) {
		t.Errorf("JSON missing total_builds, got: %s", s)
	}
}

func TestBuildReportFormatForThinker(t *testing.T) {
	report := &BuildReport{
		TotalBuilds:  20,
		ShippedCount: 15,
		FailedCount:  5,
		ShipRate:     0.75,
		LanguageBreak: map[string]int{"go": 18, "rust": 2},
		ShippedTraits: ShippedTraits{
			WithTests:  12,
			WithReadme: 14,
			TestRate:   0.8,
			ReadmeRate: 0.933,
		},
		FailureGroups: []FailureGroup{
			{Pattern: "compilation_error", Count: 3, Percentage: 0.6, Desc: "Code failed to compile"},
			{Pattern: "timeout", Count: 2, Percentage: 0.4, Desc: "Build exceeded time limit"},
		},
	}

	s := report.FormatForThinker()
	if !strings.Contains(s, "Ship rate: 75.0%") {
		t.Error("missing ship rate")
	}
	if !strings.Contains(s, "compilation_error") {
		t.Error("missing failure pattern")
	}
	if !strings.Contains(s, "go: 18") {
		t.Error("missing language breakdown")
	}
}
