package think

// Issue represents a diagnosed problem with a proposed fix.
type Issue struct {
	Title           string   `json:"issue"`
	Severity        string   `json:"severity"` // critical, high, medium, low
	RootCause       string   `json:"root_cause"`
	FixType         string   `json:"fix_type"` // kubectl, code, config, prompt, retry
	FixCommands     []string `json:"fix_commands"`
	ExpectedOutcome string   `json:"expected_outcome"`
}

// SeverityOrder maps severity to sort order (lower = more severe).
func SeverityOrder(s string) int {
	switch s {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}
