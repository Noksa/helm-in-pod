//go:build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

// foreignPodManifest is a minimal Pod that mimics a helm-in-pod created by
// another host. The labels match what CreateHelmPod produces, except `host`
// is deliberately a sentinel value that can never equal the test runner's
// os.Hostname(), so host-scoped purge must skip it.
const foreignPodManifest = `apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    host: foreign-host-purge-isolation
    %s: foreign-invocation-id
    app.kubernetes.io/managed-by: helm-in-pod
spec:
  restartPolicy: Never
  terminationGracePeriodSeconds: 0
  containers:
  - name: helm-in-pod
    image: alpine:3.20
    command: ["sh", "-c", "sleep 600"]
`

// applyForeignPod creates a pod that is labeled as if it belonged to a
// different host and waits for it to reach Running. The pod uses the same
// container name as a real plugin pod so that any logic keyed on
// hipconsts.ContainerName behaves the same way.
func applyForeignPod(name string) {
	manifest := fmt.Sprintf(foreignPodManifest, name, hipconsts.Namespace, hipconsts.LabelOperationID)
	apply := exec.Command("kubectl", "apply", "-f", "-")
	apply.Stdin = strings.NewReader(manifest)
	_, err := Run(apply)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() string {
		getCmd := exec.Command("kubectl", "get", "pod", name,
			"-n", hipconsts.Namespace,
			"-o", "jsonpath={.status.phase}")
		out, _ := Run(getCmd)
		return strings.TrimSpace(out)
	}, 60*time.Second, 2*time.Second).Should(Equal("Running"))
}

func foreignPodPresent(name string) bool {
	getCmd := exec.Command("kubectl", "get", "pod", name,
		"-n", hipconsts.Namespace,
		"-o", "name", "--ignore-not-found")
	out, _ := Run(getCmd)
	return strings.TrimSpace(out) != ""
}

func deleteForeignPod(name string) {
	cleanup := exec.Command("kubectl", "delete", "pod", name,
		"-n", hipconsts.Namespace,
		"--ignore-not-found",
		"--grace-period=0", "--force")
	_, _ = RunWithExitCode(cleanup)
}

var _ = Describe("Purge host-scope isolation", Serial, func() {
	// Both specs marked Serial because:
	//   * `helm in-pod purge` (host-scoped) sweeps own-host kept pods which
	//     might belong to other parallel specs.
	//   * `helm in-pod purge --all` is namespace-wide and would destroy
	//     every parallel spec's in-flight pod.
	// Serial guarantees we are the only spec running while purge fires.

	var foreignPod string

	BeforeEach(func() {
		foreignPod = fmt.Sprintf("foreign-pod-%s", randomString(6))
	})

	AfterEach(func() {
		deleteForeignPod(foreignPod)
		logOnFailure("")
	})

	It("host-scoped purge leaves pods belonging to other hosts intact", func() {
		By("creating a kubectl-managed pod labeled as if owned by another host")
		applyForeignPod(foreignPod)

		By("running 'helm in-pod purge' (host-scoped, no --all)")
		purge := exec.Command("helm", "in-pod", "purge")
		_, err := Run(purge)
		Expect(err).NotTo(HaveOccurred())

		By("verifying the foreign-host pod is still present")
		// Use Consistently to give any background reconcile/delete a chance
		// to fire — the pod must remain present across the window.
		Consistently(func() bool {
			return foreignPodPresent(foreignPod)
		}, 5*time.Second, 1*time.Second).Should(BeTrue(),
			"host-scoped purge must not delete pods whose host label is not the current host")
	})

	It("purge --all sweeps the namespace, removing every helm-in-pod pod", func() {
		By("creating a foreign-host pod and verifying it is Running")
		applyForeignPod(foreignPod)
		Expect(foreignPodPresent(foreignPod)).To(BeTrue())

		By("running 'helm in-pod purge --all'")
		purge := exec.Command("helm", "in-pod", "purge", "--all")
		_, err := Run(purge)
		Expect(err).NotTo(HaveOccurred())

		By("verifying the foreign pod is removed by the namespace-wide sweep")
		Eventually(func() bool {
			return foreignPodPresent(foreignPod)
		}, 60*time.Second, 2*time.Second).Should(BeFalse(),
			"purge --all is namespace-wide and must delete pods regardless of host label")
	})
})
