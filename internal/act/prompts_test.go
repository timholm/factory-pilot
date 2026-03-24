package act

import (
	"strings"
	"testing"

	"github.com/timholm/factory-pilot/internal/analyze"
)

func TestBuildEvolutionPrompt(t *testing.T) {
	report := &analyze.BuildReport{
		TotalBuilds:  100,
		ShippedCount: 70,
		FailedCount:  30,
		ShipRate:     0.7,
		ShippedTraits: analyze.ShippedTraits{
			TestRate:   0.9,
			ReadmeRate: 0.85,
		},
		FailureGroups: []analyze.FailureGroup{
			{
				Pattern:    "compilation_error",
				Count:      15,
				Percentage: 0.5,
				Desc:       "Code failed to compile",
				Examples:   []string{"syntax error at line 5"},
			},
		},
	}

	current := []PromptTemplate{
		{Name: "build.md.tmpl", Content: "Build the project..."},
	}

	prompt := buildEvolutionPrompt(report, current)

	if !strings.Contains(prompt, "Total builds: 100") {
		t.Error("prompt should contain total builds")
	}
	if !strings.Contains(prompt, "Ship rate: 70.0%") {
		t.Error("prompt should contain ship rate")
	}
	if !strings.Contains(prompt, "compilation_error") {
		t.Error("prompt should contain failure pattern")
	}
	if !strings.Contains(prompt, "build.md.tmpl") {
		t.Error("prompt should contain template name")
	}
	if !strings.Contains(prompt, "Build the project...") {
		t.Error("prompt should contain template content")
	}
}

func TestParseEvolvedTemplates(t *testing.T) {
	response := `### build.md.tmpl
` + "```" + `
Improved build template content here.
More lines of content.
` + "```" + `

### review.md.tmpl
` + "```" + `
Improved review template content.
` + "```" + `
`

	current := []PromptTemplate{
		{Name: "build.md.tmpl", Content: "old build"},
		{Name: "review.md.tmpl", Content: "old review"},
		{Name: "missing.md.tmpl", Content: "not in response"},
	}

	evolved := parseEvolvedTemplates(response, current)

	if len(evolved) != 2 {
		t.Fatalf("expected 2 evolved templates, got %d", len(evolved))
	}

	if evolved[0].Name != "build.md.tmpl" {
		t.Errorf("evolved[0].Name = %q, want build.md.tmpl", evolved[0].Name)
	}
	if !strings.Contains(evolved[0].Content, "Improved build template") {
		t.Error("evolved[0] should contain improved content")
	}

	if evolved[1].Name != "review.md.tmpl" {
		t.Errorf("evolved[1].Name = %q, want review.md.tmpl", evolved[1].Name)
	}
}

func TestParseEvolvedTemplates_NoMatch(t *testing.T) {
	response := "Some text with no code blocks matching any template names."
	current := []PromptTemplate{
		{Name: "build.md.tmpl", Content: "old"},
	}

	evolved := parseEvolvedTemplates(response, current)
	if len(evolved) != 0 {
		t.Errorf("expected 0 evolved templates, got %d", len(evolved))
	}
}

func TestParseEvolvedTemplates_EmptyCodeBlock(t *testing.T) {
	response := "### build.md.tmpl\n```\n```\n"
	current := []PromptTemplate{
		{Name: "build.md.tmpl", Content: "old"},
	}

	evolved := parseEvolvedTemplates(response, current)
	if len(evolved) != 0 {
		t.Errorf("expected 0 evolved templates for empty code block, got %d", len(evolved))
	}
}

func TestReadPromptTemplates_BadDir(t *testing.T) {
	_, err := readPromptTemplates("/nonexistent-dir-factory-pilot-test")
	if err == nil {
		t.Error("readPromptTemplates should fail on nonexistent dir")
	}
}
