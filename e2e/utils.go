//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/onsi/ginkgo/v2"
)

// Run executes cmd from the project root directory
func Run(cmd *exec.Cmd) (string, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir
	_ = os.Chdir(cmd.Dir)
	cmd.Env = os.Environ()
	ginkgo.GinkgoWriter.Printf("  → %s\n", strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "[FAIL] %s\n%s\n", strings.Join(cmd.Args, " "), string(output))
		return string(output), fmt.Errorf("%q failed: %w", strings.Join(cmd.Args, " "), err)
	}
	return string(output), nil
}

// RunWithExitCode executes cmd and returns output and exit code (doesn't fail on non-zero exit)
func RunWithExitCode(cmd *exec.Cmd) (string, int) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir
	_ = os.Chdir(cmd.Dir)
	cmd.Env = os.Environ()
	ginkgo.GinkgoWriter.Printf("  → %s\n", strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	ginkgo.GinkgoWriter.Printf("  ← exit code: %d\n", exitCode)
	return string(output), exitCode
}

// e2eResourceFlags are low resource flags used in all e2e pod commands to avoid
// scheduling failures on constrained CI runners (single-node kind clusters).
var e2eResourceFlags = []string{"--cpu-request=50m", "--cpu-limit=0", "--memory-request=64Mi", "--memory-limit=0"}

// BuildDaemonStartCommand builds a helm in-pod daemon start command with common e2e flags.
func BuildDaemonStartCommand(args ...string) *exec.Cmd {
	finalArgs := []string{"in-pod", "daemon", "start", "--copy-repo=false"}
	finalArgs = append(finalArgs, e2eResourceFlags...)
	finalArgs = append(finalArgs, args...)
	return exec.Command("helm", finalArgs...)
}

// BuildHelmInPodCommand builds a helm in-pod exec command with common flags for e2e tests
// It automatically adds --copy-repo=false and low resource requests to avoid scheduling
// failures on constrained CI runners.
//
// IMPORTANT: All arguments after "--" are joined into a single command string to ensure
// flags like "-n namespace" are passed to helm inside the pod, not consumed by the plugin.
func BuildHelmInPodCommand(args ...string) *exec.Cmd {
	// Find the "--" separator
	separatorIndex := -1
	for i, arg := range args {
		if arg == "--" {
			separatorIndex = i
			break
		}
	}

	if separatorIndex == -1 {
		// No separator found, just add common flags
		finalArgs := []string{"in-pod", "exec", "--copy-repo=false"}
		finalArgs = append(finalArgs, e2eResourceFlags...)
		finalArgs = append(finalArgs, args...)
		return exec.Command("helm", finalArgs...)
	}

	// Split args into plugin flags (before --) and command (after --)
	pluginArgs := args[:separatorIndex]
	commandArgs := args[separatorIndex+1:]

	// Build final args: helm in-pod exec --copy-repo=false <resource-flags> <plugin-flags> -- "<command>"
	finalArgs := []string{"in-pod", "exec", "--copy-repo=false"}
	finalArgs = append(finalArgs, e2eResourceFlags...)
	finalArgs = append(finalArgs, pluginArgs...)
	finalArgs = append(finalArgs, "--")

	commandStr := strings.Join(commandArgs, " ")
	finalArgs = append(finalArgs, commandStr)

	return exec.Command("helm", finalArgs...)
}

// LoadImageToKindClusterWithName loads a local docker image into the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindBinary := "kind"
	if v, ok := os.LookupEnv("KIND"); ok {
		kindBinary = v
	}
	cmd := exec.Command(kindBinary, "load", "docker-image", name, "--name", cluster)
	_, err := Run(cmd)
	return err
}

// GetProjectDir returns the project root directory
func GetProjectDir() (string, error) {
	// Get the directory of this source file
	// When running tests, we're in the e2e package
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// If we're in the e2e directory, go up one level
	if strings.HasSuffix(wd, "/e2e") {
		return strings.TrimSuffix(wd, "/e2e"), nil
	}

	// If we're already at project root, return as-is
	return wd, nil
}
