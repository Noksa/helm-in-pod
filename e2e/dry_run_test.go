//go:build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

var _ = Describe("Dry Run", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-dryrun")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
	})

	Context("exec --dry-run", func() {
		It("should print pod spec YAML without creating a pod", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--", "helm version",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			// Should contain YAML markers
			Expect(output).To(ContainSubstring("---"))
			Expect(output).To(ContainSubstring("kind: Pod"))
			Expect(output).To(ContainSubstring("apiVersion: v1"))

			// Verify no pod was actually created (use -o name to avoid stderr "No resources found" message)
			cmd = exec.Command("kubectl", "get", "pods", "-n", hipconsts.HelmInPodNamespace,
				"-l", testLabel, "-o", "name")
			podOutput, _ := Run(cmd)
			Expect(strings.TrimSpace(podOutput)).To(BeEmpty(), "No pod should be created in dry-run mode")
		})

		It("should include volumes in dry-run output", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--volume", "configmap:test-cm:/etc/config",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("test-cm"))
			Expect(output).To(ContainSubstring("configMap"))
		})

		It("should include service account in dry-run output", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--service-account", "my-custom-sa",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("my-custom-sa"))
		})
	})

	Context("daemon start --dry-run", func() {
		It("should print daemon pod spec YAML without creating a pod", func() {
			daemonName := fmt.Sprintf("dryrun-daemon-%s", randomString(6))
			cmd := BuildDaemonStartCommand(
				"--name", daemonName,
				"--labels", testLabel,
				"--dry-run",
				"-n", testNS,
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			Expect(output).To(ContainSubstring("---"))
			Expect(output).To(ContainSubstring("kind: Pod"))
			Expect(output).To(ContainSubstring("sleep infinity"))

			// Verify no daemon pod was created
			cmd = exec.Command("kubectl", "get", "pod",
				fmt.Sprintf("daemon-%s", daemonName),
				"-n", hipconsts.HelmInPodNamespace, "--no-headers")
			podOutput, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).NotTo(Equal(0), "Pod should not exist: %s", podOutput)
		})
	})
})
