//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HELM_KUBECONTEXT env var", Ordered, func() {
	var currentContext string

	BeforeAll(func() {
		out, err := Run(exec.Command("kubectl", "config", "current-context"))
		Expect(err).NotTo(HaveOccurred(), "Could not detect current kube context")
		currentContext = strings.TrimSpace(out)
		Expect(currentContext).NotTo(BeEmpty(), "Current kube context is empty")
	})

	It("succeeds when HELM_KUBECONTEXT points to the active context", func() {
		cmd := BuildHelmInPodCommand("--", "echo", "hello-from-targeted-context")
		cmd.Env = append(os.Environ(), "HELM_KUBECONTEXT="+currentContext)

		output, err := Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Output: %s", output)
		Expect(output).To(ContainSubstring("hello-from-targeted-context"))
	})

	It("fails fast when HELM_KUBECONTEXT points to a non-existent context", func() {
		cmd := BuildHelmInPodCommand("--", "echo", "should-not-run")
		cmd.Env = append(os.Environ(), "HELM_KUBECONTEXT=this-context-does-not-exist-"+currentContext)

		output, exitCode := RunWithExitCode(cmd)
		Expect(exitCode).NotTo(Equal(0), "Expected non-zero exit; output was: %s", output)
		Expect(strings.ToLower(output)).To(SatisfyAny(
			ContainSubstring("context"),
			ContainSubstring("kubeconfig"),
			ContainSubstring("not exist"),
			ContainSubstring("not found"),
		), "Error output should mention the missing context. Got: %s", output)
		Expect(output).NotTo(ContainSubstring("should-not-run"),
			"Command must not execute when the target context is invalid")
	})

	It("succeeds when HELM_KUBECONTEXT is unset (falls back to default context)", func() {
		cmd := BuildHelmInPodCommand("--", "echo", "default-context-works")
		baseEnv := os.Environ()
		filtered := baseEnv[:0]
		for _, kv := range baseEnv {
			if !strings.HasPrefix(kv, "HELM_KUBECONTEXT=") {
				filtered = append(filtered, kv)
			}
		}
		cmd.Env = filtered

		output, err := Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Output: %s", output)
		Expect(output).To(ContainSubstring("default-context-works"))
	})

	It("daemon start respects HELM_KUBECONTEXT when targeting active context", func() {
		daemonName := "kubecontext-test"
		startCmd := BuildDaemonStartCommand("--name", daemonName)
		startCmd.Env = append(os.Environ(), "HELM_KUBECONTEXT="+currentContext)
		startOut, err := Run(startCmd)
		Expect(err).NotTo(HaveOccurred(), "Output: %s", startOut)

		DeferCleanup(func() {
			stopCmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName)
			stopCmd.Env = append(os.Environ(), "HELM_KUBECONTEXT="+currentContext)
			_, _ = Run(stopCmd)
		})

		execCmd := exec.Command("helm", "in-pod", "daemon", "exec", "--name", daemonName, "--", "echo", "daemon-context-works")
		execCmd.Env = append(os.Environ(), "HELM_KUBECONTEXT="+currentContext)
		execOut, err := Run(execCmd)
		Expect(err).NotTo(HaveOccurred(), "Output: %s", execOut)
		Expect(execOut).To(ContainSubstring("daemon-context-works"))
	})
})
