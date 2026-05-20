//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

var _ = Describe("Concurrent invocation isolation", func() {
	var testLabel string

	BeforeEach(func() {
		testLabel = generateTestLabel()
	})

	AfterEach(func() {
		logOnFailure("")
		// Safety net: anything left over with this test's label must go.
		cleanup := exec.Command("kubectl", "delete", "pods",
			"-n", hipconsts.Namespace,
			"-l", testLabel,
			"--ignore-not-found",
			"--grace-period=0", "--force")
		_, _ = RunWithExitCode(cleanup)
	})

	It("does not delete a sibling pod when a concurrent exec from the same host finishes", func() {
		// Two execs with the SAME labels — isolation must therefore rely on
		// the per-process invocation-id UUID embedded in CreateHelmPod's
		// selector. If invocation-id is ever removed from DeleteHelmPods,
		// the second exec's cleanup would also kill the long-running pod
		// of the first, and this test would fail.

		dir, _ := GetProjectDir()

		By("starting the long-running exec (process A)")
		cmdA := BuildHelmInPodCommand(
			"--labels", testLabel,
			"--", "sleep 180",
		)
		cmdA.Dir = dir
		cmdA.Env = os.Environ()
		cmdA.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		var outA bytes.Buffer
		cmdA.Stdout = &outA
		cmdA.Stderr = &outA
		Expect(cmdA.Start()).To(Succeed())
		DeferCleanup(func() {
			if cmdA.Process != nil {
				_ = syscall.Kill(-cmdA.Process.Pid, syscall.SIGKILL)
				_ = cmdA.Process.Kill()
			}
		})

		By("waiting for process A's pod to reach Running")
		var podAOpID string
		Eventually(func(g Gomega) {
			getCmd := exec.Command("kubectl", "get", "pods",
				"-n", hipconsts.Namespace,
				"-l", testLabel,
				"-o", "jsonpath={range .items[*]}{.status.phase}{end}")
			out, err := Run(getCmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(strings.TrimSpace(out)).To(Equal("Running"))
		}, 120*time.Second, 2*time.Second).Should(Succeed(),
			"process A's pod must be Running before process B starts.\nhelm output:\n%s", outA.String())

		By("recording process A's pod operation-id label")
		// Use a single jsonpath query that escapes the slash in the label key.
		getOpID := exec.Command("kubectl", "get", "pods",
			"-n", hipconsts.Namespace,
			"-l", testLabel,
			"-o", "jsonpath={.items[0].metadata.labels."+strings.ReplaceAll(hipconsts.LabelOperationID, "/", "\\/")+"}")
		out, err := Run(getOpID)
		Expect(err).NotTo(HaveOccurred())
		podAOpID = strings.TrimSpace(out)
		Expect(podAOpID).NotTo(BeEmpty(),
			"process A's pod must carry an operation-id label")

		By("running a short concurrent exec (process B) with the same labels")
		cmdB := BuildHelmInPodCommand(
			"--labels", testLabel,
			"--", "echo concurrent-B-done",
		)
		cmdB.Dir = dir
		cmdB.Env = os.Environ()
		var outB bytes.Buffer
		cmdB.Stdout = &outB
		cmdB.Stderr = &outB
		Expect(cmdB.Run()).To(Succeed(),
			"process B must complete successfully:\n%s", outB.String())
		Expect(outB.String()).To(ContainSubstring("concurrent-B-done"))

		By("verifying process A's pod is still alive after process B's cleanup")
		// If the invocation-id isolation breaks, process B's startup
		// DeleteHelmPods or its deferred cleanup would have deleted process
		// A's pod. Poll for a short window to give B's cleanup time to fire.
		Consistently(func() string {
			getCmd := exec.Command("kubectl", "get", "pods",
				"-n", hipconsts.Namespace,
				"-l", hipconsts.LabelOperationID+"="+podAOpID,
				"-o", "jsonpath={.items[0].status.phase}")
			phase, _ := Run(getCmd)
			return strings.TrimSpace(phase)
		}, 10*time.Second, 1*time.Second).Should(Equal("Running"),
			"process A's pod must survive process B's lifecycle (output A:\n%s\noutput B:\n%s)",
			outA.String(), outB.String())

		By("signaling process A's group so it cleans up its own pod and exits")
		Expect(syscall.Kill(-cmdA.Process.Pid, syscall.SIGINT)).To(Succeed())
		done := make(chan error, 1)
		go func() { done <- cmdA.Wait() }()
		Eventually(done, 60*time.Second).Should(Receive(),
			"process A must exit after SIGINT.\nhelm output:\n%s", outA.String())

		By("verifying both pods are eventually gone")
		Eventually(func() string {
			getCmd := exec.Command("kubectl", "get", "pods",
				"-n", hipconsts.Namespace,
				"-l", testLabel,
				"-o", "name")
			pods, _ := Run(getCmd)
			return strings.TrimSpace(pods)
		}, 60*time.Second, 2*time.Second).Should(BeEmpty())
	})
})
