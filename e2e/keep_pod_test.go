//go:build e2e

package e2e

import (
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

var _ = Describe("--keep-pod flag", func() {
	var testLabel string

	BeforeEach(func() {
		testLabel = generateTestLabel()
	})

	AfterEach(func() {
		// Safety cleanup: delete any kept pod from this test by label so it
		// does not pollute the namespace for other tests.
		cmd := exec.Command("kubectl", "delete", "pods",
			"-n", hipconsts.Namespace,
			"-l", testLabel,
			"--ignore-not-found")
		_, _ = RunWithExitCode(cmd)
	})

	It("should keep the pod alive after exec completes successfully", func() {
		cmd := BuildHelmInPodCommand(
			"--labels", testLabel,
			"--keep-pod",
			"--", "echo hello",
		)
		output, exitCode := RunWithExitCode(cmd)
		Expect(exitCode).To(Equal(0), "output: %s", output)
		Expect(output).To(ContainSubstring("will be kept"))

		// Pod must still exist after exec returned.
		podCmd := exec.Command("kubectl", "get", "pods",
			"-n", hipconsts.Namespace,
			"-l", testLabel,
			"-o", "name")
		podOutput, err := Run(podCmd)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(podOutput)).NotTo(BeEmpty(), "pod should still be present")
	})

	It("should keep the pod alive even when the command exits non-zero", func() {
		cmd := BuildHelmInPodCommand(
			"--labels", testLabel,
			"--keep-pod",
			"--", "exit 42",
		)
		_, exitCode := RunWithExitCode(cmd)
		Expect(exitCode).To(Equal(42))

		podCmd := exec.Command("kubectl", "get", "pods",
			"-n", hipconsts.Namespace,
			"-l", testLabel,
			"-o", "name")
		podOutput, err := Run(podCmd)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(podOutput)).NotTo(BeEmpty(), "pod should be kept even on failure")
	})

	It("should remove the kept pod when the next exec runs on the same host", func() {
		// First exec: keep pod.
		firstLabel := generateTestLabel()
		cmd1 := BuildHelmInPodCommand(
			"--labels", firstLabel,
			"--keep-pod",
			"--", "echo first",
		)
		_, exitCode := RunWithExitCode(cmd1)
		Expect(exitCode).To(Equal(0))

		podCmd := exec.Command("kubectl", "get", "pods",
			"-n", hipconsts.Namespace,
			"-l", firstLabel,
			"-o", "name")
		podOutput, _ := Run(podCmd)
		Expect(strings.TrimSpace(podOutput)).NotTo(BeEmpty(), "first pod should be kept")

		// Second exec (different label, no keep-pod): CreateHelmPod deletes kept pods on startup.
		secondLabel := generateTestLabel()
		cmd2 := BuildHelmInPodCommand(
			"--labels", secondLabel,
			"--", "echo second",
		)
		_, exitCode2 := RunWithExitCode(cmd2)
		Expect(exitCode2).To(Equal(0))

		// The first pod (labeled firstLabel) should now be gone.
		checkCmd := exec.Command("kubectl", "get", "pods",
			"-n", hipconsts.Namespace,
			"-l", firstLabel,
			"-o", "name")
		remaining, _ := Run(checkCmd)
		Expect(strings.TrimSpace(remaining)).To(BeEmpty(), "kept pod should be cleaned by next exec")

		// Cleanup second pod label too (second exec deletes its own pod normally).
		exec.Command("kubectl", "delete", "pods",
			"-n", hipconsts.Namespace,
			"-l", secondLabel, "--ignore-not-found").Run()
	})

	It("should remove the kept pod when purge is run", func() {
		cmd := BuildHelmInPodCommand(
			"--labels", testLabel,
			"--keep-pod",
			"--", "echo hello",
		)
		_, exitCode := RunWithExitCode(cmd)
		Expect(exitCode).To(Equal(0))

		// Confirm pod kept.
		podCmd := exec.Command("kubectl", "get", "pods",
			"-n", hipconsts.Namespace,
			"-l", testLabel,
			"-o", "name")
		podOutput, _ := Run(podCmd)
		Expect(strings.TrimSpace(podOutput)).NotTo(BeEmpty())

		// Purge (without --all) must clean kept pods on this host.
		purgeCmd := exec.Command("helm", "in-pod", "purge")
		_, _ = RunWithExitCode(purgeCmd)

		checkCmd := exec.Command("kubectl", "get", "pods",
			"-n", hipconsts.Namespace,
			"-l", testLabel,
			"-o", "name")
		remaining, _ := Run(checkCmd)
		Expect(strings.TrimSpace(remaining)).To(BeEmpty(), "purge should clean kept pod")
	})
})
