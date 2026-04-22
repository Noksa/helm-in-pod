//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

var _ = Describe("Daemon Flags", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-daemon-flags")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
	})

	Context("daemon start --force", func() {
		var daemonName string

		BeforeEach(func() {
			daemonName = fmt.Sprintf("force-daemon-%s", randomString(6))
		})

		AfterEach(func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should fail when starting a daemon that already exists without --force", func() {
			By("starting daemon first time")
			cmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "First start should succeed: %s", output)

			By("attempting to start same daemon again without --force")
			cmd = BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).NotTo(Equal(0), "Should fail without --force, output: %s", output)
			Expect(output).To(ContainSubstring("already exists"))
		})

		It("should recreate daemon when --force is specified", func() {
			By("starting daemon first time")
			cmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "First start should succeed: %s", output)

			By("getting first pod UID")
			cmd = exec.Command("kubectl", "get", "pod",
				fmt.Sprintf("daemon-%s", daemonName),
				"-n", hipconsts.HelmInPodNamespace,
				"-o", "jsonpath={.metadata.uid}")
			firstUID, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(firstUID).NotTo(BeEmpty())

			By("force-recreating daemon")
			cmd = BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "--force", "-n", testNS)
			output, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Force start should succeed: %s", output)

			By("verifying pod was recreated with new UID")
			Eventually(func() string {
				cmd = exec.Command("kubectl", "get", "pod",
					fmt.Sprintf("daemon-%s", daemonName),
					"-n", hipconsts.HelmInPodNamespace,
					"-o", "jsonpath={.metadata.uid}")
				uid, _ := Run(cmd)
				return uid
			}).WithTimeout(60 * time.Second).ShouldNot(Equal(firstUID))
		})
	})

	Context("daemon stop idempotency", func() {
		It("should succeed when stopping a non-existent daemon", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "stop",
				"--name", fmt.Sprintf("nonexistent-%s", randomString(6)),
				"-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Stopping non-existent daemon should succeed, output: %s", output)
		})

		It("should succeed when stopping a daemon twice", func() {
			daemonName := fmt.Sprintf("stop-twice-%s", randomString(6))

			By("starting daemon")
			cmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Start should succeed: %s", output)

			By("stopping daemon first time")
			cmd = exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			output, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "First stop should succeed: %s", output)

			By("stopping daemon second time")
			cmd = exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Second stop should succeed, output: %s", output)
		})
	})

	Context("daemon --name from environment variable", func() {
		var daemonName string

		BeforeEach(func() {
			daemonName = fmt.Sprintf("envname-daemon-%s", randomString(6))
		})

		AfterEach(func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should use HELM_IN_POD_DAEMON_NAME env var when --name is not specified", func() {
			By("starting daemon with --name flag")
			cmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Start should succeed: %s", output)

			By("executing in daemon using env var instead of --name")
			// Set the env var on the current process so RunWithExitCode's os.Environ() picks it up
			_ = os.Setenv(hipconsts.EnvDaemonName, daemonName)
			defer func() { _ = os.Unsetenv(hipconsts.EnvDaemonName) }()

			cmd = exec.Command("helm", "in-pod", "daemon", "exec",
				"-n", testNS,
				"--", "echo hello-from-env")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "Should use env var for daemon name, output: %s", output)
			Expect(output).To(ContainSubstring("hello-from-env"))
		})
	})

	Context("daemon exec --update-all-repos", func() {
		var daemonName string

		BeforeEach(func() {
			By("ensuring grafana repo exists on host")
			cmd := exec.Command("helm", "repo", "add", "grafana", "https://grafana.github.io/helm-charts", "--force-update")
			_, _ = Run(cmd)

			daemonName = fmt.Sprintf("update-repos-%s", randomString(6))
			cmd = BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "--copy-repo", "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should update all repos with --update-all-repos", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--update-all-repos",
				"--", "helm search repo grafana/grafana --max-col-width 0")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("grafana/grafana"))
		})
	})
})
