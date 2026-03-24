package think

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/timholm/factory-pilot/internal/config"
	"github.com/timholm/factory-pilot/internal/diagnose"
)

const diagnosisPrompt = `You are the autonomous operations manager for a software factory pipeline.

## Current System Status
%s

## Your Job
Analyze every component. Find what's broken, degraded, or suboptimal.
For each issue, provide the EXACT fix — not vague advice.

Categories of fixes:
- kubectl: restart pods, scale deployments, patch configs
- code: edit source files in a repo to fix bugs
- prompt: improve build/seo/review prompt templates
- retry: reset failed builds with better parameters
- config: change environment variables or K8s manifests

## Output
JSON array of issues, max 10, ordered by severity:
[{
  "issue": "brief title",
  "severity": "critical|high|medium|low",
  "root_cause": "why this is happening",
  "fix_type": "kubectl|code|prompt|retry|config",
  "fix_commands": ["exact command 1", "exact command 2"],
  "expected_outcome": "what should change after fix"
}]

IMPORTANT: Return ONLY the JSON array. No markdown, no commentary.`

// Thinker uses Claude Opus with extended thinking to analyze system status.
type Thinker struct {
	cfg *config.Config
}

// NewThinker creates a new thinker.
func NewThinker(cfg *config.Config) *Thinker {
	return &Thinker{cfg: cfg}
}

// Analyze sends the full system status to Claude Opus and returns prioritized issues.
func (t *Thinker) Analyze(status *diagnose.SystemStatus) ([]Issue, error) {
	report := diagnose.FormatReport(status)
	prompt := fmt.Sprintf(diagnosisPrompt, report)

	issues, err := t.callClaude(prompt)
	if err != nil {
		return nil, fmt.Errorf("claude analysis: %w", err)
	}

	// Sort by severity
	sort.Slice(issues, func(i, j int) bool {
		return SeverityOrder(issues[i].Severity) < SeverityOrder(issues[j].Severity)
	})

	// Cap at max fixes
	if len(issues) > t.cfg.MaxFixes {
		issues = issues[:t.cfg.MaxFixes]
	}

	return issues, nil
}

// callClaude invokes the Claude CLI with the analysis prompt.
func (t *Thinker) callClaude(prompt string) ([]Issue, error) {
	cmd := exec.Command(t.cfg.ClaudeBinary,
		"-p", prompt,
		"--model", "opus",
		"--max-turns", "3",
		"--output-format", "text",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude exec: %s: %w", stderr.String(), err)
	}

	return parseResponse(stdout.String())
}

// parseResponse extracts the JSON array from Claude's response.
func parseResponse(raw string) ([]Issue, error) {
	// Find JSON array in response — Claude might wrap it in markdown
	text := strings.TrimSpace(raw)

	// Strip markdown code fences if present
	if idx := strings.Index(text, "["); idx >= 0 {
		text = text[idx:]
	}
	if idx := strings.LastIndex(text, "]"); idx >= 0 {
		text = text[:idx+1]
	}

	var issues []Issue
	if err := json.Unmarshal([]byte(text), &issues); err != nil {
		return nil, fmt.Errorf("parse issues JSON: %w (raw: %s)", err, truncate(raw, 500))
	}

	return issues, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
