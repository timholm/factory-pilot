package diagnose

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// K8sCollector gathers Kubernetes pod health via kubectl.
type K8sCollector struct {
	namespace string
}

// NewK8sCollector creates a K8s collector for the given namespace.
func NewK8sCollector(namespace string) *K8sCollector {
	return &K8sCollector{namespace: namespace}
}

// CollectPods returns the status of all pods in the namespace.
func (k *K8sCollector) CollectPods(ctx context.Context) ([]PodStatus, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", k.namespace,
		"-o", "custom-columns=NAME:.metadata.name,STATUS:.status.phase,RESTARTS:.status.containerStatuses[0].restartCount,AGE:.metadata.creationTimestamp,READY:.status.containerStatuses[0].ready",
		"--no-headers")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("kubectl get pods: %s: %w", stderr.String(), err)
	}

	var pods []PodStatus
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		restarts, _ := strconv.Atoi(fields[2])
		ready := fields[4] == "true"

		pods = append(pods, PodStatus{
			Name:      fields[0],
			Namespace: k.namespace,
			Status:    fields[1],
			Restarts:  restarts,
			Age:       fields[3],
			Ready:     ready,
		})
	}

	return pods, nil
}

// GetPodLogs returns the last N lines of logs for a pod.
func (k *K8sCollector) GetPodLogs(ctx context.Context, podName string, lines int) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "logs",
		"-n", k.namespace,
		podName,
		"--tail", strconv.Itoa(lines))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("kubectl logs %s: %s: %w", podName, stderr.String(), err)
	}

	return stdout.String(), nil
}
