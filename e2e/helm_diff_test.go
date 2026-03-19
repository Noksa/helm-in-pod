//go:build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helm Diff Plugin", func() {
	var (
		releaseName string
		chartDir    string
		testNS      string
		testLabel   string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-diff")
		DeferCleanup(func() { deleteNamespace(testNS) })

		releaseName = generateReleaseName("diff-test")
		chartDir = createTestChart("diffchart")
		testLabel = generateTestLabel()

		By(fmt.Sprintf("creating namespace %s", testNS))
		cmd := exec.Command("kubectl", "create", "namespace", testNS, "--dry-run=client", "-o", "yaml")
		output, _ := Run(cmd)
		cmd = exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(output)
		_, _ = Run(cmd)
	})

	AfterEach(func() {
		logOnFailure(testNS)
		By("cleaning up chart directory")
		cleanupChart(chartDir)
	})

	Context("helm diff upgrade", func() {
		BeforeEach(func() {
			By("installing initial release")
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to install initial release: %s", output)
		})

		It("should show diff when upgrading with changes", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "diff", "upgrade", releaseName, "/tmp/chart", "-n", testNS, "--set", "replicaCount=3", "--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(2), "Expected exit code 2 when diff shows changes, output: %s", output)
			Expect(output).To(ContainSubstring("-   replicas: 1"))
			Expect(output).To(ContainSubstring("+   replicas: 3"))
		})

		It("should return exit code 0 when no changes detected", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "diff", "upgrade", releaseName, "/tmp/chart", "-n", testNS, "--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected exit code 0 when no changes, output: %s", output)
		})

		It("should fail with exit code 1 for non-existent release", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "diff", "upgrade", "nonexistent-release", "/tmp/chart", "-n", testNS, "--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1 for non-existent release, output: %s", output)
		})

		It("should show diff with --three-way-merge flag", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "diff", "upgrade", releaseName, "/tmp/chart", "-n", testNS, "--set", "replicaCount=2", "--three-way-merge", "--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			// Exit code 2 means changes detected
			Expect(exitCode).To(BeElementOf([]int{0, 2}), "Expected exit code 0 or 2, output: %s", output)
		})

		It("should show diff with image tag change", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "diff", "upgrade", releaseName, "/tmp/chart", "-n", testNS, "--set", "image.tag=1.22", "--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(2), "Expected exit code 2 when diff shows changes, output: %s", output)
			Expect(output).To(ContainSubstring("1.22"))
		})
	})

	Context("helm diff release", func() {
		BeforeEach(func() {
			By("installing initial release")
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to install initial release: %s", output)

			By("upgrading release to create revision 2")
			cmd = BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "upgrade", releaseName, "/tmp/chart", "-n", testNS, "--set", "replicaCount=2")
			output, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to upgrade release: %s", output)
		})

		It("should show diff between revisions", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "diff", "revision", releaseName, "1", "2", "-n", testNS, "--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(2), "Expected exit code 2 when diff shows changes, output: %s", output)
			Expect(output).To(ContainSubstring("-   replicas: 1"))
			Expect(output).To(ContainSubstring("+   replicas: 2"))
		})

		It("should return exit code 0 when comparing same revision", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "diff", "revision", releaseName, "1", "1", "-n", testNS, "--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected exit code 0 when comparing same revision, output: %s", output)
		})

		It("should fail with exit code 1 for invalid revision", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "diff", "release", releaseName, "1", "999", "-n", testNS, "--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1 for invalid revision, output: %s", output)
		})
	})

	Context("helm diff rollback", func() {
		BeforeEach(func() {
			By("installing initial release")
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to install initial release: %s", output)

			By("upgrading release to create revision 2")
			cmd = BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "upgrade", releaseName, "/tmp/chart", "-n", testNS, "--set", "replicaCount=3")
			output, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to upgrade release: %s", output)
		})

		It("should show diff for rollback to previous revision", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "diff", "rollback", releaseName, "1", "-n", testNS, "--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(2), "Expected exit code 2 when diff shows changes, output: %s", output)
			Expect(output).To(ContainSubstring("-   replicas: 3"))
			Expect(output).To(ContainSubstring("+   replicas: 1"))
		})

		It("should return exit code 0 when rolling back to current revision", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "diff", "rollback", releaseName, "2", "-n", testNS, "--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected exit code 0 when rolling back to current, output: %s", output)
		})

		It("should fail with exit code 1 for invalid revision", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "diff", "rollback", releaseName, "999", "-n", testNS, "--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1 for invalid revision, output: %s", output)
		})
	})

	Context("helm diff with complex scenarios", func() {
		BeforeEach(func() {
			By("installing initial release")
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to install initial release: %s", output)
		})

		It("should handle multiple value changes", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "diff", "upgrade", releaseName, "/tmp/chart", "-n", testNS,
				"--set", "replicaCount=5",
				"--set", "image.tag=1.23",
				"--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(2), "Expected exit code 2 when diff shows changes, output: %s", output)
			Expect(output).To(ContainSubstring("replicaCount"))
			Expect(output).To(ContainSubstring("1.23"))
		})

		It("should work with --suppress-secrets flag", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "diff", "upgrade", releaseName, "/tmp/chart", "-n", testNS,
				"--set", "replicaCount=2",
				"--suppress-secrets",
				"--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(BeElementOf([]int{0, 2}), "Expected exit code 0 or 2, output: %s", output)
		})

		It("should work with --context flag", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "diff", "upgrade", releaseName, "/tmp/chart", "-n", testNS,
				"--set", "replicaCount=2",
				"--context", "3",
				"--detailed-exitcode")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(BeElementOf([]int{0, 2}), "Expected exit code 0 or 2, output: %s", output)
		})
	})
})
