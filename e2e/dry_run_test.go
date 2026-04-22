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

		It("should include user labels in dry-run output", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--labels", "team=platform,env=dev",
				"--dry-run",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("team: platform"))
			Expect(output).To(ContainSubstring("env: dev"))
		})

		It("should include annotations in dry-run output", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--annotations", "note=hello,owner=qa",
				"--dry-run",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("note: hello"))
			Expect(output).To(ContainSubstring("owner: qa"))
		})

		It("should include tolerations in dry-run output", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--tolerations", "dedicated=gpu:NoSchedule:Equal",
				"--dry-run",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("tolerations:"))
			Expect(output).To(ContainSubstring("key: dedicated"))
			Expect(output).To(ContainSubstring("value: gpu"))
			Expect(output).To(ContainSubstring("effect: NoSchedule"))
			Expect(output).To(ContainSubstring("operator: Equal"))
		})

		It("should include node selector in dry-run output", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--node-selector", "disktype=ssd",
				"--dry-run",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("nodeSelector:"))
			Expect(output).To(ContainSubstring("disktype: ssd"))
		})

		It("should include host network in dry-run output when enabled", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--host-network",
				"--dry-run",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("hostNetwork: true"))
		})

		It("should omit host network in dry-run output by default", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).NotTo(ContainSubstring("hostNetwork: true"))
		})

		It("should include run-as-user and run-as-group in dry-run output", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--run-as-user", "1000",
				"--run-as-group", "2000",
				"--dry-run",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("runAsUser: 1000"))
			Expect(output).To(ContainSubstring("runAsGroup: 2000"))
		})

		It("should include image pull secret in dry-run output", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--image-pull-secret", "my-registry-secret",
				"--dry-run",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("imagePullSecrets:"))
			Expect(output).To(ContainSubstring("name: my-registry-secret"))
		})

		It("should include pull policy in dry-run output", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--pull-policy", "Always",
				"--dry-run",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("imagePullPolicy: Always"))
		})

		It("should include custom image in dry-run output", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--image", "my-registry.io/custom:v9.9.9",
				"--dry-run",
				"--", "echo test",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("image: my-registry.io/custom:v9.9.9"))
		})
	})

	Context("daemon start --dry-run flag propagation", func() {
		It("should include user labels in daemon dry-run output", func() {
			daemonName := fmt.Sprintf("dryrun-daemon-%s", randomString(6))
			cmd := BuildDaemonStartCommand(
				"--name", daemonName,
				"--labels", testLabel,
				"--labels", "team=platform",
				"--dry-run",
				"-n", testNS,
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("team: platform"))
			// daemon label is added by daemon start itself
			Expect(output).To(ContainSubstring(fmt.Sprintf("daemon: %s", daemonName)))
		})

		It("should include tolerations in daemon dry-run output", func() {
			daemonName := fmt.Sprintf("dryrun-daemon-%s", randomString(6))
			cmd := BuildDaemonStartCommand(
				"--name", daemonName,
				"--labels", testLabel,
				"--tolerations", "key=value:NoSchedule:Equal",
				"--dry-run",
				"-n", testNS,
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("tolerations:"))
			Expect(output).To(ContainSubstring("effect: NoSchedule"))
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
