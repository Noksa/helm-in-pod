//go:build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Timeout Flag", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-timeout")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
	})

	Context("--timeout with exec", func() {
		It("should complete before timeout when command finishes quickly", func() {
			start := time.Now()
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--timeout", "2m",
				"--", "echo", "fast-command",
			)
			output, exitCode := RunWithExitCode(cmd)
			elapsed := time.Since(start)

			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("fast-command"))
			Expect(elapsed).To(BeNumerically("<", 2*time.Minute), "Should complete well before timeout")
		})

		It("should terminate command that exceeds timeout", func() {
			start := time.Now()
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--timeout", "10s",
				"--", "sleep", "300",
			)
			output, exitCode := RunWithExitCode(cmd)
			elapsed := time.Since(start)

			Expect(exitCode).NotTo(Equal(0), "Should fail due to timeout, output: %s", output)
			// Allow generous time for pod creation + timeout + cleanup
			Expect(elapsed).To(BeNumerically("<", 5*time.Minute),
				"Should not run for the full sleep duration")
		})
	})

	Context("--timeout with daemon exec", func() {
		var daemonName string

		BeforeEach(func() {
			daemonName = fmt.Sprintf("timeout-daemon-%s", randomString(6))
			cmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should complete before timeout in daemon mode", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--timeout", "2m",
				"--", "echo daemon-fast")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("daemon-fast"))
		})
	})
})
