//go:build e2e

package e2e

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Active Deadline Seconds Flag", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-deadline")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
	})

	Context("--dry-run verification", func() {
		It("should include activeDeadlineSeconds in pod spec when flag is set", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--active-deadline-seconds", "1800",
				"--", "helm version",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("activeDeadlineSeconds: 1800"))
		})

		It("should not include activeDeadlineSeconds in pod spec when flag is not set (default 0)", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--", "helm version",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).NotTo(ContainSubstring("activeDeadlineSeconds"),
				"activeDeadlineSeconds should not appear in pod spec when not set")
		})

		It("should include activeDeadlineSeconds in daemon pod spec when flag is set", func() {
			daemonName := generateReleaseName("deadline-daemon")
			cmd := BuildDaemonStartCommand(
				"--name", daemonName,
				"--labels", testLabel,
				"--dry-run",
				"--active-deadline-seconds", "3600",
				"-n", testNS,
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("activeDeadlineSeconds: 3600"))
		})

		It("should not create a pod when dry-run is combined with active-deadline-seconds", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--active-deadline-seconds", "60",
				"--", "helm version",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			// Verify no pod was created
			cmd = exec.Command("kubectl", "get", "pods", "-n", hipconsts.HelmInPodNamespace,
				"-l", strings.Replace(testLabel, "=", "=", 1), "-o", "name")
			podOutput, _ := Run(cmd)
			Expect(strings.TrimSpace(podOutput)).To(BeEmpty(), "No pod should be created in dry-run mode")
		})
	})

	Context("--active-deadline-seconds runtime enforcement", func() {
		It("should complete successfully when command finishes before the deadline", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--active-deadline-seconds", "300",
				"--", "echo deadline-ok",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("deadline-ok"))
		})

		It("should terminate pod when active deadline expires before command completes", func(_ context.Context) {
			start := time.Now()
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--active-deadline-seconds", "5",
				"--timeout", "2m",
				"--", "sleep 300",
			)
			output, exitCode := RunWithExitCode(cmd)
			elapsed := time.Since(start)

			// Pod must be terminated (non-zero exit), not run for full sleep duration
			Expect(exitCode).NotTo(Equal(0),
				"Pod should be terminated by Kubernetes deadline, output: %s", output)
			// Should terminate well before the sleep command would finish
			Expect(elapsed).To(BeNumerically("<", 3*time.Minute),
				"Pod should be terminated by deadline, not run for full duration")
		}, NodeTimeout(5*time.Minute))
	})
})
