//go:build e2e

package e2e

import (
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Daemon Status and List", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-daemon-sl")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
	})

	Context("daemon list", func() {
		It("should show 'No daemon pods found' when none exist", Serial, func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "list")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("No daemon pods found"))
		})

		It("should list running daemon pods", func() {
			daemonName := fmt.Sprintf("list-daemon-%s", randomString(6))
			cmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)
			defer func() {
				cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
				_, _ = Run(cmd)
			}()

			cmd = exec.Command("helm", "in-pod", "daemon", "list")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring(daemonName))
			Expect(output).To(ContainSubstring("NAME"))
			Expect(output).To(ContainSubstring("PHASE"))
		})

		It("should list multiple daemon pods", func() {
			daemon1 := fmt.Sprintf("list-d1-%s", randomString(6))
			daemon2 := fmt.Sprintf("list-d2-%s", randomString(6))

			cmd := BuildDaemonStartCommand("--name", daemon1, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon1: %s", output)

			cmd = BuildDaemonStartCommand("--name", daemon2, "--labels", testLabel, "-n", testNS)
			output, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon2: %s", output)

			defer func() {
				cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemon1, "-n", testNS)
				_, _ = Run(cmd)
				cmd = exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemon2, "-n", testNS)
				_, _ = Run(cmd)
			}()

			cmd = exec.Command("helm", "in-pod", "daemon", "list")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring(daemon1))
			Expect(output).To(ContainSubstring(daemon2))
		})

		It("should support 'ls' alias for list", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "ls")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
		})
	})

	Context("daemon status", func() {
		var daemonName string

		BeforeEach(func() {
			daemonName = fmt.Sprintf("status-daemon-%s", randomString(6))
			cmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should show status of a running daemon", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "status", "--name", daemonName)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring(daemonName))
			Expect(output).To(ContainSubstring("Running"))
			Expect(output).To(ContainSubstring("PROPERTY"))
			Expect(output).To(ContainSubstring("VALUE"))
		})

		It("should show image in status output", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "status", "--name", daemonName)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("Image"))
		})

		It("should fail for non-existent daemon", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "status",
				"--name", fmt.Sprintf("nonexistent-%s", randomString(6)))
			_, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).NotTo(Equal(0))
		})
	})
})
