package act

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/timholm/factory-pilot/internal/config"
	"github.com/timholm/factory-pilot/internal/think"
)

// ActionResult records what happened when we tried to fix an issue.
type ActionResult struct {
	Issue     think.Issue `json:"issue"`
	Executed  bool        `json:"executed"`
	Output    string      `json:"output"`
	Error     string      `json:"error,omitempty"`
	Duration  string      `json:"duration"`
	Timestamp time.Time   `json:"timestamp"`
}

// Executor applies fixes identified by the thinker.
type Executor struct {
	cfg     *config.Config
	kubectl *KubectlRunner
	claude  *ClaudeRunner
	retry   *RetryRunner
}

// NewExecutor creates an executor.
func NewExecutor(cfg *config.Config) *Executor {
	return &Executor{
		cfg:     cfg,
		kubectl: NewKubectlRunner(cfg.K8sNamespace),
		claude:  NewClaudeRunner(cfg.ClaudeBinary, cfg.FactoryGitDir),
		retry:   NewRetryRunner(cfg.FactoryDataDir),
	}
}

// Execute runs fixes for each issue. Returns results for every issue.
func (e *Executor) Execute(issues []think.Issue) []ActionResult {
	var results []ActionResult

	for i, issue := range issues {
		if i >= e.cfg.MaxFixes {
			log.Printf("[executor] reached max fixes (%d), stopping", e.cfg.MaxFixes)
			break
		}

		log.Printf("[executor] [%d/%d] %s (severity=%s, type=%s)",
			i+1, len(issues), issue.Title, issue.Severity, issue.FixType)

		result := e.executeOne(issue)
		results = append(results, result)

		if result.Error != "" {
			log.Printf("[executor] FAILED: %s", result.Error)
		} else {
			log.Printf("[executor] OK (%s)", result.Duration)
		}
	}

	return results
}

func (e *Executor) executeOne(issue think.Issue) ActionResult {
	start := time.Now()
	result := ActionResult{
		Issue:     issue,
		Timestamp: start,
	}

	if e.cfg.DryRun {
		result.Executed = false
		result.Output = fmt.Sprintf("[DRY RUN] would execute %d commands for: %s",
			len(issue.FixCommands), issue.Title)
		for _, cmd := range issue.FixCommands {
			result.Output += fmt.Sprintf("\n  > %s", cmd)
		}
		result.Duration = time.Since(start).String()
		return result
	}

	result.Executed = true
	var outputs []string

	for _, cmd := range issue.FixCommands {
		var out string
		var err error

		switch issue.FixType {
		case "kubectl":
			out, err = e.kubectl.Run(cmd)
		case "code":
			out, err = e.claude.RunCodeFix(cmd)
		case "prompt":
			out, err = e.claude.RunPromptFix(cmd)
		case "retry":
			out, err = e.retry.Run(cmd)
		case "config":
			out, err = e.kubectl.Run(cmd) // config changes go through kubectl
		default:
			err = fmt.Errorf("unknown fix type: %s", issue.FixType)
		}

		if err != nil {
			result.Error = err.Error()
			result.Output = strings.Join(outputs, "\n")
			result.Duration = time.Since(start).String()
			return result
		}
		outputs = append(outputs, out)
	}

	result.Output = strings.Join(outputs, "\n")
	result.Duration = time.Since(start).String()
	return result
}
