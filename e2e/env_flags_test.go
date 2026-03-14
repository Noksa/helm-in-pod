//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Environment Variable Flags", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-env")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
	})

	Context("--env flag", func() {
		It("should inject a single environment variable into the pod", func() {
			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--env", "MY_VAR=hello_world",
				"--", "sh -c 'echo $MY_VAR'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("hello_world"))
		})

		It("should inject multiple environment variables", func() {
			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--env", "FOO=bar",
				"--env", "BAZ=qux",
				"--", "sh -c 'echo ${FOO}-${BAZ}'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("bar-qux"))
		})

		It("should handle environment variable with special characters in value", func() {
			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--env", "SPECIAL=hello world & more",
				"--", "sh -c 'echo \"$SPECIAL\"'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("hello world & more"))
		})

		It("should handle environment variable with empty value", func() {
			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--env", "EMPTY_VAR=",
				"--", "sh -c 'echo \"empty=${EMPTY_VAR}end\"'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("empty=end"))
		})
	})

	Context("--subst-env flag", func() {
		It("should substitute environment variable from host", func() {
			// Set a known env var on the host
			testValue := fmt.Sprintf("subst-test-%s", randomString(8))
			_ = os.Setenv("HIP_TEST_SUBST", testValue)
			defer func() { _ = os.Unsetenv("HIP_TEST_SUBST") }()

			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--subst-env", "HIP_TEST_SUBST",
				"--", "sh -c 'echo $HIP_TEST_SUBST'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring(testValue))
		})

		It("should substitute multiple environment variables from host", func() {
			_ = os.Setenv("HIP_VAR_A", "alpha")
			_ = os.Setenv("HIP_VAR_B", "beta")
			defer func() {
				_ = os.Unsetenv("HIP_VAR_A")
				_ = os.Unsetenv("HIP_VAR_B")
			}()

			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--subst-env", "HIP_VAR_A",
				"--subst-env", "HIP_VAR_B",
				"--", "sh -c 'echo ${HIP_VAR_A}-${HIP_VAR_B}'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("alpha-beta"))
		})

		It("should handle unset host variable as empty", func() {
			_ = os.Unsetenv("HIP_NONEXISTENT_VAR")

			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--subst-env", "HIP_NONEXISTENT_VAR",
				"--", "sh -c 'echo \"val=${HIP_NONEXISTENT_VAR}end\"'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("val=end"))
		})
	})

	Context("--env and --subst-env combined", func() {
		It("should support both flags together", func() {
			_ = os.Setenv("HIP_HOST_VAR", "from-host")
			defer func() { _ = os.Unsetenv("HIP_HOST_VAR") }()

			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--env", "EXPLICIT=from-flag",
				"--subst-env", "HIP_HOST_VAR",
				"--", "sh -c 'echo ${EXPLICIT}-${HIP_HOST_VAR}'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("from-flag-from-host"))
		})
	})

	Context("--env in daemon mode", func() {
		var daemonName string

		BeforeEach(func() {
			daemonName = fmt.Sprintf("env-daemon-%s", randomString(6))
			cmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should inject env vars via daemon exec", func() {
			// Use printenv instead of echo $VAR to avoid the variable appearing
			// unquoted in the echo log line (which runs before exports in the wrapped script)
			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--env", "DAEMON_VAR=daemon_value",
				"--", "printenv DAEMON_VAR")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("daemon_value"))
		})

		It("should substitute host env vars via daemon exec", func() {
			_ = os.Setenv("HIP_DAEMON_SUBST", "daemon-host-val")
			defer func() { _ = os.Unsetenv("HIP_DAEMON_SUBST") }()

			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--subst-env", "HIP_DAEMON_SUBST",
				"--", "printenv HIP_DAEMON_SUBST")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("daemon-host-val"))
		})
	})
})
