package analyze

import "regexp"

// Pattern represents a known failure pattern matched against error logs.
type Pattern struct {
	Name    string
	Regex   *regexp.Regexp
	Desc    string
	Matches int
}

// DefaultPatterns returns the standard set of error log patterns to detect.
func DefaultPatterns() []*Pattern {
	return []*Pattern{
		{
			Name:  "no_test_files",
			Regex: regexp.MustCompile(`(?i)(no test files|no tests found|test.*not found)`),
			Desc:  "Build has no test files or tests were not found",
		},
		{
			Name:  "wrong_module_path",
			Regex: regexp.MustCompile(`(?i)(module.*does not match|go\.mod.*module|cannot find module)`),
			Desc:  "Go module path is incorrect or mismatched",
		},
		{
			Name:  "compilation_error",
			Regex: regexp.MustCompile(`(?i)(cannot compile|build failed|syntax error|undefined:|cannot refer)`),
			Desc:  "Code failed to compile",
		},
		{
			Name:  "test_failure",
			Regex: regexp.MustCompile(`(?i)(FAIL\s|test.*failed|panic.*test|--- FAIL)`),
			Desc:  "Tests exist but fail",
		},
		{
			Name:  "missing_readme",
			Regex: regexp.MustCompile(`(?i)(README.*not found|missing README|no README)`),
			Desc:  "README.md is missing from the build output",
		},
		{
			Name:  "missing_dependency",
			Regex: regexp.MustCompile(`(?i)(missing go\.sum|cannot find package|no required module|unresolved import)`),
			Desc:  "Missing or unresolved dependency",
		},
		{
			Name:  "timeout",
			Regex: regexp.MustCompile(`(?i)(timeout|timed out|deadline exceeded|context deadline)`),
			Desc:  "Build or test exceeded time limit",
		},
		{
			Name:  "permission_denied",
			Regex: regexp.MustCompile(`(?i)(permission denied|access denied|EACCES)`),
			Desc:  "File system permission error",
		},
		{
			Name:  "empty_output",
			Regex: regexp.MustCompile(`(?i)(no output|empty.*output|produced no files)`),
			Desc:  "Build produced no output or empty files",
		},
		{
			Name:  "lint_failure",
			Regex: regexp.MustCompile(`(?i)(lint.*fail|golangci|vet.*fail|staticcheck)`),
			Desc:  "Linting or static analysis failed",
		},
	}
}

// MatchPatterns runs all patterns against an error log and returns those that match.
func MatchPatterns(patterns []*Pattern, errorLog string) []*Pattern {
	var matched []*Pattern
	for _, p := range patterns {
		if p.Regex.MatchString(errorLog) {
			matched = append(matched, p)
		}
	}
	return matched
}
