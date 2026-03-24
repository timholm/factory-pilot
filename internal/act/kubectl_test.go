package act

import (
	"sort"
	"testing"
)

func TestKubectlRunner_Validate_AllowedVerbs(t *testing.T) {
	k := NewKubectlRunner("factory")

	allowed := []string{
		"kubectl get pods",
		"kubectl describe pod/worker-abc",
		"kubectl logs pod/worker-abc --tail 100",
		"kubectl rollout restart deployment/worker",
		"kubectl scale deployment/worker --replicas=3",
		"kubectl patch deployment/worker -p '{\"spec\":{\"replicas\":2}}'",
		"kubectl apply -f deployment.yaml",
		"kubectl set image deployment/worker worker=image:v2",
		"kubectl annotate pod/worker-abc note=fixed",
		"kubectl label pod/worker-abc tier=backend",
	}

	for _, cmd := range allowed {
		t.Run(cmd, func(t *testing.T) {
			if err := k.validate(cmd); err != nil {
				t.Errorf("validate(%q) returned error: %v", cmd, err)
			}
		})
	}
}

func TestKubectlRunner_Validate_BlockedVerbs(t *testing.T) {
	k := NewKubectlRunner("factory")

	blocked := []string{
		"kubectl delete pod/worker-abc",
		"kubectl exec pod/worker-abc -- bash",
		"kubectl edit deployment/worker",
		"kubectl create namespace evil",
		"kubectl drain node/worker-1",
		"kubectl cordon node/worker-1",
		"kubectl taint node/worker-1 key=value:NoSchedule",
	}

	for _, cmd := range blocked {
		t.Run(cmd, func(t *testing.T) {
			if err := k.validate(cmd); err == nil {
				t.Errorf("validate(%q) should have returned error for blocked verb", cmd)
			}
		})
	}
}

func TestKubectlRunner_Validate_BlockedPatterns(t *testing.T) {
	k := NewKubectlRunner("factory")

	blocked := []string{
		"kubectl scale deployment/worker --replicas=0 --force",
		"kubectl patch deployment/worker --grace-period=0",
		"kubectl get pods --all-namespaces",
	}

	for _, cmd := range blocked {
		t.Run(cmd, func(t *testing.T) {
			if err := k.validate(cmd); err == nil {
				t.Errorf("validate(%q) should have returned error for blocked pattern", cmd)
			}
		})
	}
}

func TestKubectlRunner_Validate_TooShort(t *testing.T) {
	k := NewKubectlRunner("factory")

	if err := k.validate("kubectl"); err == nil {
		t.Error("validate should reject single-word commands")
	}
}

func TestKubectlRunner_Run_NotKubectl(t *testing.T) {
	k := NewKubectlRunner("factory")

	_, err := k.Run("rm -rf /")
	if err == nil {
		t.Error("Run should reject non-kubectl commands")
	}
}

func TestKubectlRunner_Run_EmptyCommand(t *testing.T) {
	k := NewKubectlRunner("factory")

	_, err := k.Run("")
	if err == nil {
		t.Error("Run should reject empty commands")
	}
}

func TestNewKubectlRunner(t *testing.T) {
	k := NewKubectlRunner("test-ns")
	if k == nil {
		t.Fatal("NewKubectlRunner returned nil")
	}
	if k.namespace != "test-ns" {
		t.Errorf("namespace = %q, want test-ns", k.namespace)
	}
}

func TestAllowedVerbs(t *testing.T) {
	verbs := allowedVerbs()
	if len(verbs) == 0 {
		t.Fatal("allowedVerbs() returned empty slice")
	}

	// Check that expected verbs are present
	sort.Strings(verbs)
	expected := []string{"apply", "get", "scale", "patch", "rollout", "describe", "logs", "set", "annotate", "label"}
	for _, exp := range expected {
		found := false
		for _, v := range verbs {
			if v == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected verb %q not in allowedVerbs()", exp)
		}
	}
}

func TestBlockedPatterns_Coverage(t *testing.T) {
	// Ensure all blocked patterns are actually checked
	if len(blockedPatterns) == 0 {
		t.Fatal("blockedPatterns is empty")
	}

	k := NewKubectlRunner("factory")
	for _, pattern := range blockedPatterns {
		// Construct a command that uses an allowed verb but includes the blocked pattern
		cmd := "kubectl get pods " + pattern
		err := k.validate(cmd)
		if err == nil {
			t.Errorf("validate should block pattern %q", pattern)
		}
	}
}
