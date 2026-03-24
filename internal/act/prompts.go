package act

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/timholm/factory-pilot/internal/analyze"
)

// PromptTemplate represents a prompt template file on disk.
type PromptTemplate struct {
	Name    string
	Content string
}

// EvolvePrompts reads current prompts, sends them with build analysis to Claude,
// gets improved prompts, writes them to disk, commits, and pushes.
func EvolvePrompts(buildReport *analyze.BuildReport, factoryRepoPath string, claudeBinary string) error {
	promptsDir := filepath.Join(factoryRepoPath, "prompts")

	// Read current prompt templates.
	current, err := readPromptTemplates(promptsDir)
	if err != nil {
		return fmt.Errorf("read current prompts: %w", err)
	}

	if len(current) == 0 {
		return fmt.Errorf("no prompt templates found in %s", promptsDir)
	}

	// Build the evolution prompt for Claude.
	evolutionPrompt := buildEvolutionPrompt(buildReport, current)

	// Call Claude to generate improved prompts.
	cmd := exec.Command(claudeBinary,
		"-p", evolutionPrompt,
		"--max-turns", "5",
		"--output-format", "text",
	)
	cmd.Dir = factoryRepoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude evolve: %s: %w", stderr.String(), err)
	}

	responseText := stdout.String()

	// Parse evolved templates from the response.
	evolved := parseEvolvedTemplates(responseText, current)
	if len(evolved) == 0 {
		return fmt.Errorf("claude returned no parseable improved templates")
	}

	// Write evolved prompts to the factory repo.
	for _, tmpl := range evolved {
		destPath := filepath.Join(promptsDir, tmpl.Name)
		if err := os.WriteFile(destPath, []byte(tmpl.Content), 0o644); err != nil {
			return fmt.Errorf("write evolved prompt %s: %w", tmpl.Name, err)
		}
	}

	// Git add, commit, push.
	if out, err := runGit(factoryRepoPath, "add", "prompts/"); err != nil {
		return fmt.Errorf("git add: %s: %w", out, err)
	}

	var names []string
	for _, tmpl := range evolved {
		names = append(names, tmpl.Name)
	}
	commitMsg := fmt.Sprintf("factory-pilot: evolve prompts based on build analysis\n\nShip rate: %.1f%%, updated: %s",
		buildReport.ShipRate*100, strings.Join(names, ", "))

	if out, err := runGit(factoryRepoPath, "commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("git commit: %s: %w", out, err)
	}

	if out, err := runGit(factoryRepoPath, "push"); err != nil {
		return fmt.Errorf("git push: %s: %w", out, err)
	}

	return nil
}

func buildEvolutionPrompt(report *analyze.BuildReport, current []PromptTemplate) string {
	var sb strings.Builder

	sb.WriteString("You are an expert prompt engineer for an autonomous code factory that generates Go repositories.\n\n")
	sb.WriteString("## Current Build Performance\n\n")
	sb.WriteString(fmt.Sprintf("- Total builds: %d\n", report.TotalBuilds))
	sb.WriteString(fmt.Sprintf("- Ship rate: %.1f%%\n", report.ShipRate*100))
	sb.WriteString(fmt.Sprintf("- Failed: %d builds\n\n", report.FailedCount))

	sb.WriteString("## Top Failure Patterns\n\n")
	for _, fg := range report.FailureGroups {
		sb.WriteString(fmt.Sprintf("- **%s** (%d occurrences, %.1f%%): %s\n",
			fg.Pattern, fg.Count, fg.Percentage*100, fg.Desc))
		for _, ex := range fg.Examples {
			sb.WriteString(fmt.Sprintf("  Example: `%s`\n", ex))
		}
	}

	sb.WriteString("\n## Shipped Build Traits\n\n")
	sb.WriteString(fmt.Sprintf("- Test rate: %.1f%%\n", report.ShippedTraits.TestRate*100))
	sb.WriteString(fmt.Sprintf("- README rate: %.1f%%\n", report.ShippedTraits.ReadmeRate*100))

	sb.WriteString("\n## Current Prompt Templates\n\n")
	for _, tmpl := range current {
		sb.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", tmpl.Name, tmpl.Content))
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("Generate improved versions of each prompt template that address the failure patterns above.\n")
	sb.WriteString("Focus on:\n")
	sb.WriteString("1. Explicit instructions that prevent the top failure patterns\n")
	sb.WriteString("2. Reinforcing patterns that lead to shipped builds (tests, README, correct module path)\n")
	sb.WriteString("3. Adding guard rails and verification steps\n\n")
	sb.WriteString("Output each improved template in a fenced code block with the filename as a header.\n")
	sb.WriteString("Also provide a brief summary of what changed and why.\n")

	return sb.String()
}

// parseEvolvedTemplates extracts template content from Claude's markdown response.
func parseEvolvedTemplates(response string, current []PromptTemplate) []PromptTemplate {
	var evolved []PromptTemplate

	for _, tmpl := range current {
		// Look for the template name followed by a code block.
		nameIdx := strings.Index(response, tmpl.Name)
		if nameIdx == -1 {
			continue
		}

		// Find the next code block after the template name.
		remaining := response[nameIdx:]
		blockStart := strings.Index(remaining, "```")
		if blockStart == -1 {
			continue
		}

		// Skip the opening ``` and any language tag.
		afterFence := remaining[blockStart+3:]
		newlineIdx := strings.Index(afterFence, "\n")
		if newlineIdx == -1 {
			continue
		}
		contentStart := afterFence[newlineIdx+1:]

		// Find the closing ```.
		blockEnd := strings.Index(contentStart, "```")
		if blockEnd == -1 {
			continue
		}

		content := strings.TrimSpace(contentStart[:blockEnd])
		if content != "" {
			evolved = append(evolved, PromptTemplate{
				Name:    tmpl.Name,
				Content: content,
			})
		}
	}

	return evolved
}

// readPromptTemplates reads all .md.tmpl files from the given directory.
func readPromptTemplates(dir string) ([]PromptTemplate, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read prompt dir %s: %w", dir, err)
	}

	var templates []PromptTemplate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if ext != ".tmpl" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", name, err)
		}
		templates = append(templates, PromptTemplate{
			Name:    name,
			Content: string(content),
		})
	}

	return templates, nil
}
