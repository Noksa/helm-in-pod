//go:build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Service Account", func() {
	var (
		testNS    string
		testLabel string
		saName    string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-sa")
		testLabel = generateTestLabel()
		saName = fmt.Sprintf("test-sa-%s", randomString(6))

		// Create a service account in the helm-in-pod namespace
		cmd := exec.Command("kubectl", "create", "serviceaccount", saName,
			"-n", "helm-in-pod")
		output, err := Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create SA: %s", output)

		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
		cmd := exec.Command("kubectl", "delete", "serviceaccount", saName,
			"-n", "helm-in-pod", "--ignore-not-found")
		_, _ = Run(cmd)
	})

	Context("exec mode", func() {
		It("should use custom service account in the pod", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--service-account", saName,
				"--",
				"sh -c 'cat /var/run/secrets/kubernetes.io/serviceaccount/namespace'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("helm-in-pod"))
		})
	})

	Context("daemon mode", func() {
		var daemonName string

		BeforeEach(func() {
			daemonName = fmt.Sprintf("sa-daemon-%s", randomString(6))
			cmd := BuildDaemonStartCommand(
				"--name", daemonName,
				"--labels", testLabel,
				"--service-account", saName,
				"-n", testNS,
			)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should use custom service account in daemon pod", func() {
			cmd := exec.Command("kubectl", "get", "pod",
				fmt.Sprintf("daemon-%s", daemonName),
				"-n", "helm-in-pod",
				"-o", "jsonpath={.spec.serviceAccountName}")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(output)).To(Equal(saName))
		})
	})
})
