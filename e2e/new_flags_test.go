//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

var _ = Describe("New flags and env vars", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-newflags")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
	})

	// -------------------------------------------------------------------------
	// HELM_IN_POD_IMAGE
	// -------------------------------------------------------------------------

	Context("HELM_IN_POD_IMAGE env var", func() {
		It("should use the env var image for actual pod execution (no --image flag required)", func() {
			// Set the env var to the well-known default image so the kind cluster
			// has it cached, but pass it via env var instead of --image.
			// We use --keep-pod so we can inspect the live pod's image field.
			defaultImage := "docker.io/noksa/kubectl-helm:v1.34.5-v4.1.1"

			cmd := BuildHelmInPodCommand("--labels", testLabel, "--keep-pod", "--", "echo ok")
			cmd.Env = append(os.Environ(), fmt.Sprintf("HELM_IN_POD_IMAGE=%s", defaultImage))
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("ok"))

			DeferCleanup(func() {
				exec.Command("kubectl", "delete", "pods",
					"-n", hipconsts.Namespace, "-l", testLabel, "--ignore-not-found").Run()
			})

			// Verify the live pod is running the image from the env var, not a hardcoded default
			podCmd := exec.Command("kubectl", "get", "pods",
				"-n", hipconsts.Namespace,
				"-l", testLabel,
				"-o", "jsonpath={.items[0].spec.containers[0].image}")
			podOutput, err := Run(podCmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(podOutput)).To(Equal(defaultImage))
		})

		It("should allow --image to override HELM_IN_POD_IMAGE at runtime", func() {
			// Even with env var set, an explicit --image flag must win.
			// Both images must be available in the kind cluster; we use the same
			// image with a different reference format to force the override to be visible.
			defaultImage := "docker.io/noksa/kubectl-helm:v1.34.5-v4.1.1"

			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--keep-pod",
				"--image", defaultImage,
				"--", "echo override-ok",
			)
			cmd.Env = append(os.Environ(), "HELM_IN_POD_IMAGE=this-image-does-not-exist:latest")
			output, exitCode := RunWithExitCode(cmd)
			// If the env var were honored over the flag, the non-existent image
			// would cause a pod startup failure (ImagePullBackOff). Exit 0 confirms
			// the explicit flag took precedence.
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("override-ok"))

			DeferCleanup(func() {
				exec.Command("kubectl", "delete", "pods",
					"-n", hipconsts.Namespace, "-l", testLabel, "--ignore-not-found").Run()
			})
		})
	})

	// -------------------------------------------------------------------------
	// HELM_IN_POD_NAMESPACE
	// -------------------------------------------------------------------------

	Context("HELM_IN_POD_NAMESPACE env var", func() {
		It("should create the pod in the custom namespace", func() {
			customNS := fmt.Sprintf("hip-e2e-ns-%s", randomString(6))
			DeferCleanup(func() {
				exec.Command("kubectl", "delete", "namespace", customNS, "--ignore-not-found", "--wait=false").Run()
			})

			// --keep-pod so the pod is still there when we inspect it
			cmd := BuildHelmInPodCommand("--labels", testLabel, "--keep-pod", "--", "echo ns-ok")
			cmd.Env = append(os.Environ(), fmt.Sprintf("HELM_IN_POD_NAMESPACE=%s", customNS))
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("ns-ok"))

			// Pod must be in the custom namespace, not the default helm-in-pod one
			podInCustom := exec.Command("kubectl", "get", "pods",
				"-n", customNS, "-l", testLabel, "-o", "name")
			podOutput, err := Run(podInCustom)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(podOutput)).NotTo(BeEmpty(),
				"pod should exist in custom namespace %s", customNS)

			podInDefault := exec.Command("kubectl", "get", "pods",
				"-n", hipconsts.Namespace, "-l", testLabel, "-o", "name")
			defaultOutput, _ := Run(podInDefault)
			Expect(strings.TrimSpace(defaultOutput)).To(BeEmpty(),
				"pod should NOT be in the default namespace")
		})

		It("should create namespace-scoped resources (SA, CRB) in the custom namespace", func() {
			customNS := fmt.Sprintf("hip-e2e-ns-%s", randomString(6))
			DeferCleanup(func() {
				exec.Command("kubectl", "delete", "namespace", customNS, "--ignore-not-found", "--wait=false").Run()
				exec.Command("kubectl", "delete", "clusterrolebinding", customNS, "--ignore-not-found").Run()
			})

			cmd := BuildHelmInPodCommand("--labels", testLabel, "--", "echo sa-ok")
			cmd.Env = append(os.Environ(), fmt.Sprintf("HELM_IN_POD_NAMESPACE=%s", customNS))
			_, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0))

			// The plugin should have created a ServiceAccount in the custom namespace
			saCmd := exec.Command("kubectl", "get", "serviceaccount", customNS, "-n", customNS, "-o", "name")
			saOutput, err := Run(saCmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(saOutput)).NotTo(BeEmpty(),
				"ServiceAccount should exist in custom namespace")
		})
	})

	// -------------------------------------------------------------------------
	// --startup-timeout
	// -------------------------------------------------------------------------

	Context("--startup-timeout flag", func() {
		It("should fail fast when timeout is shorter than any pod startup time", func() {
			// 100ms is always shorter than the first readiness poll cycle (1s period).
			// The plugin must time out and return a non-zero exit code quickly —
			// well under the 5-minute default — proving the flag is wired into
			// waitUntilPodIsRunning rather than being silently ignored.
			start := time.Now()
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--startup-timeout=100ms",
				"--", "echo hello",
			)
			output, exitCode := RunWithExitCode(cmd)
			elapsed := time.Since(start)

			Expect(exitCode).NotTo(Equal(0), "should fail due to startup timeout")
			Expect(output).To(ContainSubstring("timeout"))
			// Must fail well within 60 s — if the default 5 min were used instead
			// this assertion would fail on the first CI run.
			Expect(elapsed).To(BeNumerically("<", 60*time.Second),
				"startup-timeout should have fired quickly, not after the 5-minute default")
		})

		It("should succeed when startup-timeout is generous enough", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--startup-timeout=10m",
				"--", "echo timeout-ok",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("timeout-ok"))
		})
	})
})
