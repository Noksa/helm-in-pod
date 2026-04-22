//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Helm In Pod E2E Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// This runs ONLY on process 1 - do one-time setup here
	By("verifying cluster access")
	cmd := exec.Command("kubectl", "cluster-info")
	_, err := Run(cmd)
	if err != nil {
		scriptDir, _ := GetProjectDir()
		kubeconfigPath := fmt.Sprintf("%s/e2e/.kubeconfig", scriptDir)
		Skip(fmt.Sprintf("Cannot access cluster. Run: ./e2e/setup-cluster.sh (kubeconfig: %s)", kubeconfigPath))
	}

	By("building the plugin binary")
	cmd = exec.Command("make", "build")
	output, err := Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build plugin binary: %s", output)

	By("installing plugin to helm")
	// Uninstall if exists
	cmd = exec.Command("helm", "plugin", "uninstall", "in-pod")
	_, _ = Run(cmd)

	// Install using make install-local
	cmd = exec.Command("make", "install-local")
	output, err = Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to install helm plugin: %s", output)

	By("verifying plugin installation")
	cmd = exec.Command("helm", "plugin", "list")
	output, err = Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, output).To(ContainSubstring("in-pod"), "Plugin not found in helm plugin list. Output: %s", output)

	By("installing helm diff plugin")
	cmd = exec.Command("helm", "plugin", "list")
	output, _ = Run(cmd)
	if !strings.Contains(output, "diff") {
		cmd = exec.Command("helm", "plugin", "install", "https://github.com/databus23/helm-diff")
		_, err = Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to install helm diff plugin")
	}

	By("creating helm-in-pod namespace for executor pods")
	cmd = exec.Command("kubectl", "create", "namespace", hipconsts.HelmInPodNamespace, "--dry-run=client", "-o", "yaml")
	output, _ = Run(cmd)
	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(output)
	_, _ = Run(cmd)

	return nil // No data to pass to other processes
}, func(data []byte) {
	// This runs on ALL processes - set defaults for each process
	SetDefaultEventuallyTimeout(30 * time.Second)
	SetDefaultEventuallyPollingInterval(500 * time.Millisecond)
	SetDefaultConsistentlyDuration(5 * time.Second)
	SetDefaultConsistentlyPollingInterval(500 * time.Millisecond)

	// Set KUBECONFIG for this process
	scriptDir, err := GetProjectDir()
	Expect(err).NotTo(HaveOccurred())
	kubeconfigPath := fmt.Sprintf("%s/e2e/.kubeconfig", scriptDir)

	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		Skip(fmt.Sprintf("Kubeconfig not found at %s. Run: ./e2e/setup-cluster.sh", kubeconfigPath))
	}

	_ = os.Setenv("KUBECONFIG", kubeconfigPath)
})

var _ = SynchronizedAfterSuite(func() {
	// This runs on ALL processes - nothing to do per-process
}, func() {
	// This runs ONLY on process 1 after all other processes finish
	By("cleaning up helm-in-pod namespace")
	cmd := exec.Command("kubectl", "delete", "namespace", hipconsts.HelmInPodNamespace, "--ignore-not-found", "--wait=false")
	_, _ = Run(cmd)

	By("uninstalling helm plugin")
	cmd = exec.Command("helm", "plugin", "uninstall", "in-pod")
	_, _ = Run(cmd)
})
