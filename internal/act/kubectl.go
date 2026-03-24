package act

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// allowedKubectlVerbs is the whitelist of safe kubectl operations.
// No delete, no force, no exec — only observe and heal.
var allowedKubectlVerbs = map[string]bool{
	"get":       true,
	"describe":  true,
	"logs":      true,
	"rollout":   true,
	"scale":     true,
	"patch":     true,
	"apply":     true,
	"set":       true,
	"annotate":  true,
	"label":     true,
}

// blockedPatterns are substrings that should never appear in commands.
var blockedPatterns = []string{
	"--force",
	"--grace-period=0",
	"delete namespace",
	"delete node",
	"delete pv",
	"--all-namespaces",
}

// KubectlRunner executes safe kubectl commands.
type KubectlRunner struct {
	namespace string
}

// NewKubectlRunner creates a kubectl runner for the given namespace.
func NewKubectlRunner(namespace string) *KubectlRunner {
	return &KubectlRunner{namespace: namespace}
}

// Run executes a kubectl command after safety validation.
func (k *KubectlRunner) Run(command string) (string, error) {
	if err := k.validate(command); err != nil {
		return "", err
	}

	// Parse the command — it should start with "kubectl"
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	if parts[0] != "kubectl" {
		return "", fmt.Errorf("not a kubectl command: %s", parts[0])
	}

	// Inject namespace if not already specified
	hasNs := false
	for _, p := range parts {
		if p == "-n" || p == "--namespace" {
			hasNs = true
			break
		}
	}
	if !hasNs {
		parts = append(parts[:2], append([]string{"-n", k.namespace}, parts[2:]...)...)
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("kubectl: %s: %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

func (k *KubectlRunner) validate(command string) error {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		return fmt.Errorf("kubectl command too short")
	}

	verb := parts[1]
	if !allowedKubectlVerbs[verb] {
		return fmt.Errorf("kubectl verb %q not allowed (whitelist: %v)", verb, allowedVerbs())
	}

	lower := strings.ToLower(command)
	for _, blocked := range blockedPatterns {
		if strings.Contains(lower, blocked) {
			return fmt.Errorf("command contains blocked pattern: %s", blocked)
		}
	}

	return nil
}

func allowedVerbs() []string {
	var verbs []string
	for v := range allowedKubectlVerbs {
		verbs = append(verbs, v)
	}
	return verbs
}
