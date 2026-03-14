//go:build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helm Repository Sync", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-repo")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
	})

	Context("when --copy-repo is enabled", func() {
		BeforeEach(func() {
			By("ensuring grafana repo exists on host")
			cmd := exec.Command("helm", "repo", "add", "grafana", "https://grafana.github.io/helm-charts", "--force-update")
			_, _ = Run(cmd)
		})

		It("should copy helm repositories from host to pod", func() {
			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--", "helm", "repo", "list")
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful repo list, output: %s", output)
			Expect(output).To(ContainSubstring("grafana"), "Expected grafana repo to be copied")
		})

		It("should update helm repositories when --update-repo is specified", func() {
			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo",
				"--update-repo", "grafana"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--", "helm", "search", "repo", "grafana/grafana", "--versions", "--max-col-width", "0")
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful search, output: %s", output)
			Expect(output).To(ContainSubstring("grafana/grafana"), "Expected to find grafana chart after repo update")
		})

		It("should be able to install chart from repository", func() {
			releaseName := generateReleaseName("repo-test")

			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo",
				"--update-repo", "grafana"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--", "helm", "install", releaseName, "grafana/grafana",
				"-n", testNS,
				"--version", "8.0.0",
				"--set", "replicas=1",
				"--wait", "--timeout=2m")
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful install from repo, output: %s", output)
			Expect(output).To(ContainSubstring("STATUS: deployed"))

			By("cleaning up release")
			cleanupArgs := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			cleanupArgs = append(cleanupArgs, e2eResourceFlags...)
			cleanupArgs = append(cleanupArgs, "--", "helm", "uninstall", releaseName, "-n", testNS)
			cmd = exec.Command("helm", cleanupArgs...)
			_, _ = Run(cmd)
		})
	})

	Context("when --copy-repo is disabled", func() {
		It("should not have access to host helm repositories", func() {
			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--", "helm", "repo", "list")
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			// helm repo list returns exit code 1 when no repos exist ("no repositories to show"),
			// so we accept both 0 and 1 as valid outcomes here.
			Expect(exitCode).To(BeNumerically("<=", 1), "Expected repo list to run, output: %s", output)
			Expect(output).NotTo(ContainSubstring("grafana"), "Should not have grafana repo when copy-repo is false")
		})

		It("should fail to install chart from repository", func() {
			releaseName := generateReleaseName("repo-fail-test")

			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--", "helm", "install", releaseName, "grafana/grafana", "-n", testNS)
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).NotTo(Equal(0), "Expected install to fail without repo, output: %s", output)
			Expect(strings.ToLower(output)).To(Or(
				ContainSubstring("not found"),
				ContainSubstring("no repository"),
			), "Expected error about missing repository")
		})
	})

	Context("when using daemon mode with --copy-repo", func() {
		var daemonName string

		BeforeEach(func() {
			By("ensuring grafana repo exists on host")
			cmd := exec.Command("helm", "repo", "add", "grafana", "https://grafana.github.io/helm-charts", "--force-update")
			_, _ = Run(cmd)

			daemonName = fmt.Sprintf("repo-daemon-%s", randomString(6))

			By(fmt.Sprintf("starting daemon %s with --copy-repo", daemonName))
			cmd = BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "--copy-repo", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)
		})

		AfterEach(func() {
			By(fmt.Sprintf("stopping daemon %s", daemonName))
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should have helm repositories available in daemon pod", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--", "helm", "repo", "list")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful repo list in daemon, output: %s", output)
			Expect(output).To(ContainSubstring("grafana"), "Expected grafana repo in daemon pod")
		})

		It("should be able to search charts in daemon pod", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--", "helm", "search", "repo", "grafana/grafana")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful search in daemon, output: %s", output)
			Expect(output).To(ContainSubstring("grafana/grafana"), "Expected to find grafana chart in daemon")
		})
	})

	Context("when using daemon mode without --copy-repo", func() {
		var daemonName string

		BeforeEach(func() {
			By("ensuring grafana repo exists on host")
			cmd := exec.Command("helm", "repo", "add", "grafana", "https://grafana.github.io/helm-charts", "--force-update")
			_, _ = Run(cmd)

			daemonName = fmt.Sprintf("repo-daemon-%s", randomString(6))

			By(fmt.Sprintf("starting daemon %s with --copy-repo=false", daemonName))
			cmd = BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "--copy-repo=false", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)
		})

		AfterEach(func() {
			By(fmt.Sprintf("stopping daemon %s", daemonName))
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should start daemon successfully without repo copy", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--", "echo", "daemon is running")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful command in daemon, output: %s", output)
			Expect(output).To(ContainSubstring("daemon is running"))
		})

		It("should not have helm repositories when started without --copy-repo", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--", "helm", "repo", "list")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Expected successful repo list in daemon, output: %s", output)
			Expect(output).NotTo(ContainSubstring("grafana"), "Should not have grafana repo when daemon started without --copy-repo")
		})
	})
})
