//go:build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helm Commands", func() {
	var (
		releaseName string
		chartDir    string
		testNS      string
		testLabel   string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-helm")
		DeferCleanup(func() { deleteNamespace(testNS) })

		releaseName = generateReleaseName("test-release")
		chartDir = createTestChart("testchart")
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
		deleteNamespace(testNS)
		By("cleaning up chart directory")
		cleanupChart(chartDir)
	})

	Context("helm install", func() {
		It("should successfully install a chart", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful install, output: %s", output)
			Expect(output).To(ContainSubstring("STATUS: deployed"))
		})

		It("should fail with exit code 1 when installing duplicate release", func() {
			By("installing release first time")
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "First install should succeed, output: %s", output)

			By("attempting to install same release again")
			cmd = BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, exitCode = RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1 for duplicate release, output: %s", output)
			Expect(output).To(ContainSubstring("cannot reuse a name that is still in use"))
		})

		It("should fail with exit code 1 for invalid chart path", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "install", releaseName, "/nonexistent/chart", "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1 for invalid chart, output: %s", output)
		})
	})

	Context("helm upgrade", func() {
		BeforeEach(func() {
			By("installing initial release")
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to install initial release: %s", output)
		})

		It("should successfully upgrade a release", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "upgrade", releaseName, "/tmp/chart", "-n", testNS, "--set", "replicaCount=2")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful upgrade, output: %s", output)
			Expect(output).To(ContainSubstring("STATUS: deployed"))
		})

		It("should fail with exit code 1 when upgrading non-existent release", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "upgrade", "nonexistent-release", "/tmp/chart", "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1 for non-existent release, output: %s", output)
		})

		It("should succeed with --install flag for new release", func() {
			newRelease := generateReleaseName("new-release")
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "upgrade", "--install", newRelease, "/tmp/chart", "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful upgrade --install, output: %s", output)

			By("cleaning up new release")
			cmd = BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "uninstall", newRelease, "-n", testNS)
			_, _ = Run(cmd)
		})
	})

	Context("helm list", func() {
		BeforeEach(func() {
			By("installing a release")
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to install release: %s", output)
		})

		It("should list installed releases", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "list", "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful list, output: %s", output)
			Expect(output).To(ContainSubstring(releaseName))
		})

		It("should return exit code 0 for empty namespace", func() {
			emptyNS := fmt.Sprintf("empty-ns-%s", randomString(6))
			cmd := exec.Command("kubectl", "create", "namespace", emptyNS)
			_, _ = Run(cmd)
			defer func() {
				cmd := exec.Command("kubectl", "delete", "namespace", emptyNS)
				_, _ = Run(cmd)
			}()

			cmd = BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "list", "-n", emptyNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected exit code 0 for empty namespace, output: %s", output)
		})
	})

	Context("helm status", func() {
		BeforeEach(func() {
			By("installing a release")
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to install release: %s", output)
		})

		It("should show status of installed release", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "status", releaseName, "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful status, output: %s", output)
			Expect(output).To(ContainSubstring("STATUS: deployed"))
		})

		It("should fail with exit code 1 for non-existent release", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "status", "nonexistent-release", "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1 for non-existent release, output: %s", output)
		})
	})

	Context("helm uninstall", func() {
		BeforeEach(func() {
			By("installing a release")
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to install release: %s", output)
		})

		It("should successfully uninstall a release", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "uninstall", releaseName, "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful uninstall, output: %s", output)
		})

		It("should fail with exit code 1 when uninstalling non-existent release", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "uninstall", "nonexistent-release", "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1 for non-existent release, output: %s", output)
		})
	})

	Context("helm get", func() {
		BeforeEach(func() {
			By("installing a release")
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--copy", fmt.Sprintf("%s:/tmp/chart", chartDir), "--", "helm", "install", releaseName, "/tmp/chart", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to install release: %s", output)
		})

		It("should get values of installed release", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "get", "values", releaseName, "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful get values, output: %s", output)
		})

		It("should get manifest of installed release", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "get", "manifest", releaseName, "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful get manifest, output: %s", output)
			Expect(output).To(ContainSubstring("kind: Deployment"))
		})

		It("should fail with exit code 1 for non-existent release", func() {
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "helm", "get", "values", "nonexistent-release", "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(1), "Expected exit code 1 for non-existent release, output: %s", output)
		})
	})
})
