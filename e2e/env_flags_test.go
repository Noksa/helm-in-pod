//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--env", "MY_VAR=hello_world",
				"--", "sh -c 'echo $MY_VAR'")
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("hello_world"))
		})

		It("should inject multiple environment variables", func() {
			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--env", "FOO=bar",
				"--env", "BAZ=qux",
				"--", "sh -c 'echo ${FOO}-${BAZ}'")
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("bar-qux"))
		})

		It("should handle environment variable with special characters in value", func() {
			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--env", "SPECIAL=hello world & more",
				"--", "sh -c 'echo \"$SPECIAL\"'")
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("hello world & more"))
		})

		It("should handle environment variable with empty value", func() {
			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--env", "EMPTY_VAR=",
				"--", "sh -c 'echo \"empty=${EMPTY_VAR}end\"'")
			cmd := exec.Command("helm", args...)
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

			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--subst-env", "HIP_TEST_SUBST",
				"--", "sh -c 'echo $HIP_TEST_SUBST'")
			cmd := exec.Command("helm", args...)
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

			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--subst-env", "HIP_VAR_A",
				"--subst-env", "HIP_VAR_B",
				"--", "sh -c 'echo ${HIP_VAR_A}-${HIP_VAR_B}'")
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("alpha-beta"))
		})

		It("should handle unset host variable as empty", func() {
			_ = os.Unsetenv("HIP_NONEXISTENT_VAR")

			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--subst-env", "HIP_NONEXISTENT_VAR",
				"--", "sh -c 'echo \"val=${HIP_NONEXISTENT_VAR}end\"'")
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("val=end"))
		})
	})

	Context("--env and --subst-env combined", func() {
		It("should support both flags together", func() {
			_ = os.Setenv("HIP_HOST_VAR", "from-host")
			defer func() { _ = os.Unsetenv("HIP_HOST_VAR") }()

			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args, "--env", "EXPLICIT=from-flag",
				"--subst-env", "HIP_HOST_VAR",
				"--", "sh -c 'echo ${EXPLICIT}-${HIP_HOST_VAR}'")
			cmd := exec.Command("helm", args...)
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

	Context("--env-file flag", func() {
		var tmpDir string

		BeforeEach(func() {
			tmpDir = GinkgoT().TempDir()
		})

		It("should load env vars from a file into the pod", func() {
			envFile := filepath.Join(tmpDir, "test.env")
			Expect(os.WriteFile(envFile, []byte("FILE_VAR=loaded-from-file\n"), 0644)).To(Succeed())

			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--env-file", envFile,
				"--", "sh -c 'echo $FILE_VAR'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("loaded-from-file"))
		})

		It("should load multiple files when --env-file is repeated", func() {
			file1 := filepath.Join(tmpDir, "a.env")
			file2 := filepath.Join(tmpDir, "b.env")
			Expect(os.WriteFile(file1, []byte("VAR_A=alpha\n"), 0644)).To(Succeed())
			Expect(os.WriteFile(file2, []byte("VAR_B=beta\n"), 0644)).To(Succeed())

			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--env-file", file1,
				"--env-file", file2,
				"--", "sh -c 'echo ${VAR_A}-${VAR_B}'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("alpha-beta"))
		})

		It("should give explicit --env priority over --env-file for the same key", func() {
			envFile := filepath.Join(tmpDir, "base.env")
			Expect(os.WriteFile(envFile, []byte("PRIORITY_VAR=from-file\n"), 0644)).To(Succeed())

			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--env-file", envFile,
				"--env", "PRIORITY_VAR=from-flag",
				"--", "sh -c 'echo $PRIORITY_VAR'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			// explicit --env wins
			Expect(output).To(ContainSubstring("from-flag"))
			Expect(output).NotTo(ContainSubstring("from-file"))
		})

		It("should support comments and quoted values in the env file", func() {
			envFile := filepath.Join(tmpDir, "complex.env")
			content := "# this is a comment\nQUOTED_VAR=\"hello world\"\nUNQUOTED=simple\n"
			Expect(os.WriteFile(envFile, []byte(content), 0644)).To(Succeed())

			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--env-file", envFile,
				"--", "sh -c 'echo \"${QUOTED_VAR}|${UNQUOTED}\"'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("hello world|simple"))
		})

		It("should load env-file vars in daemon exec mode", func() {
			daemonName := fmt.Sprintf("envfile-d-%s", randomString(6))
			startCmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			_, err := Run(startCmd)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS).Run()
			})

			envFile := filepath.Join(tmpDir, "daemon.env")
			Expect(os.WriteFile(envFile, []byte("DAEMON_FILE_VAR=daemon-loaded\n"), 0644)).To(Succeed())

			execCmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName, "-n", testNS,
				"--env-file", envFile,
				"--", "printenv DAEMON_FILE_VAR")
			output, exitCode := RunWithExitCode(execCmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("daemon-loaded"))
		})
	})

	Context("--suppress-secrets flag", func() {
		It("should mask --set values in the plugin log when flag is set", func() {
			// The command uses --set (as part of a helm call that ignores it gracefully).
			// With --suppress-secrets the plugin's own "Running '...' command" log line
			// must show [REDACTED] instead of the secret value.
			// Using `sh -c "echo done" --set password=topsecret` — sh ignores the
			// extra positional args after the command string, so exit code is 0 and
			// the echo output "done" does not contain the secret.
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--suppress-secrets",
				"--", `sh -c "echo done" --set password=topsecret`,
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("[REDACTED]"))
			Expect(output).NotTo(ContainSubstring("topsecret"))
		})

		It("should show --set values in the log when flag is not set", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--", `sh -c "echo done" --set password=visiblesecret`,
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("visiblesecret"))
			Expect(output).NotTo(ContainSubstring("[REDACTED]"))
		})

		It("should mask multiple --set flags", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--suppress-secrets",
				"--", `sh -c "echo done" --set key1=secret1 --set-string key2=secret2`,
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).NotTo(ContainSubstring("secret1"))
			Expect(output).NotTo(ContainSubstring("secret2"))
			// Both flags should produce [REDACTED]
			Expect(strings.Count(output, "[REDACTED]")).To(BeNumerically(">=", 2))
		})

		It("should mask --set values in daemon exec log", func() {
			daemonName := fmt.Sprintf("suppress-d-%s", randomString(6))
			startCmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			_, err := Run(startCmd)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS).Run()
			})

			execCmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName, "-n", testNS,
				"--suppress-secrets",
				"--", `sh -c "echo done" --set db_password=daemon_secret`)
			output, exitCode := RunWithExitCode(execCmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("[REDACTED]"))
			Expect(output).NotTo(ContainSubstring("daemon_secret"))
		})
	})
})
