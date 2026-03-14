//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Exit Code Propagation", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-exitcode")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
	})

	Context("when running simple commands", func() {
		It("should return exit code 0 for successful command", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "sh", "-c", "exit 0")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected exit code 0, output: %s", output)
		})

		It("should return exit code 1 for failed command", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "sh -c 'exit 1'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1, output: %s", output)
		})

		It("should return exit code 2 for command with exit 2", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "sh -c 'exit 2'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(2), "Expected exit code 2, output: %s", output)
		})

		It("should return exit code 42 for custom exit code", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "sh -c 'exit 42'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(42), "Expected exit code 42, output: %s", output)
		})

		It("should return exit code 127 for command not found", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "nonexistent-command-xyz")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(127), "Expected exit code 127 for command not found, output: %s", output)
		})
	})

	Context("when running commands that fail", func() {
		It("should propagate exit code from false command", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "false")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1 from false, output: %s", output)
		})

		It("should propagate exit code from failing script", func() {
			// Create a temporary script file
			scriptPath := fmt.Sprintf("/tmp/test-script-%s.sh", randomString(8))
			script := `#!/bin/sh
echo "Starting script"
echo "This will fail"
exit 5
`
			// Write script to file
			writeCmd := exec.Command("sh", "-c", fmt.Sprintf("cat > %s << 'EOF'\n%s\nEOF", scriptPath, script))
			_, err := Run(writeCmd)
			Expect(err).NotTo(HaveOccurred())

			// Make script executable
			chmodCmd := exec.Command("chmod", "+x", scriptPath)
			_, err = Run(chmodCmd)
			Expect(err).NotTo(HaveOccurred())

			// Clean up script file after test
			defer func() {
				_ = os.RemoveAll(scriptPath)
			}()

			// Copy script to pod and execute it
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy", fmt.Sprintf("%s:/tmp/test-script.sh", scriptPath),
				"--",
				"sh", "/tmp/test-script.sh",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(5), "Expected exit code 5, output: %s", output)
		})
	})

	Context("when running in daemon mode", func() {
		var daemonPodName string
		var daemonName string

		BeforeEach(func() {
			// Generate unique daemon name for parallel test isolation
			daemonName = fmt.Sprintf("test-daemon-%s", randomString(6))

			By(fmt.Sprintf("starting daemon %s without --copy-repo", daemonName))
			cmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)

			// Extract pod name from output
			expectedPodName := fmt.Sprintf("daemon-%s", daemonName)
			cmd = exec.Command("kubectl", "get", "pod", expectedPodName, "-n", hipconsts.HelmInPodNamespace, "-o", "jsonpath={.metadata.name}")
			daemonPodName, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(daemonPodName).NotTo(BeEmpty(), "Daemon pod not found")

			By(fmt.Sprintf("waiting for daemon pod %s to be ready", daemonPodName))
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "pod", daemonPodName, "-n", hipconsts.HelmInPodNamespace, "-o", "jsonpath={.status.phase}")
				phase, _ := Run(cmd)
				return phase
			}).WithTimeout(60 * time.Second).Should(Equal("Running"))
		})

		AfterEach(func() {
			By(fmt.Sprintf("stopping daemon %s", daemonName))
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should return exit code 0 for successful command in daemon", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "exec", "--name", daemonName, "-n", testNS, "--", "sh -c 'exit 0'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected exit code 0, output: %s", output)
		})

		It("should return exit code 1 for failed command in daemon", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "exec", "--name", daemonName, "-n", testNS, "--", "sh -c 'exit 1'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1, output: %s", output)
		})

		It("should return exit code 3 for custom exit code in daemon", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "exec", "--name", daemonName, "-n", testNS, "--", "sh -c 'exit 3'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(3), "Expected exit code 3, output: %s", output)
		})
	})
})
