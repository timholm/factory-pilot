package act

import (
	"bytes"
	"fmt"
	"os/exec"
)

// RebuildImage runs a Docker buildx build and pushes to GHCR.
func RebuildImage(repoPath, imageName string) error {
	fullImage := fmt.Sprintf("ghcr.io/timholm/%s:latest", imageName)

	cmd := exec.Command("docker", "buildx", "build",
		"--platform", "linux/arm64",
		"-t", fullImage,
		"--push",
		".",
	)
	cmd.Dir = repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker buildx: %s: %w", stderr.String(), err)
	}

	return nil
}
