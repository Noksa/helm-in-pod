package hippod

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

// makeLabeledPod constructs a pod in hipconsts.Namespace with the given name
// and labels. The phase is set to Running so the deleter uses the default
// grace period (matching production behavior).
func makeLabeledPod(name string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: hipconsts.Namespace,
			Labels:    labels,
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

// createPods materializes a list of pods through the fake client so the
// label-selector path is exercised end-to-end.
func createPods(cs *fake.Clientset, pods ...*corev1.Pod) {
	for _, p := range pods {
		_, err := cs.CoreV1().Pods(hipconsts.Namespace).Create(
			context.Background(), p, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	}
}

// listPodNames returns the sorted names of pods remaining in the namespace.
func listPodNames(cs *fake.Clientset) []string {
	list, err := cs.CoreV1().Pods(hipconsts.Namespace).List(
		context.Background(), metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		names = append(names, list.Items[i].Name)
	}
	return names
}

var _ = Describe("DeleteHelmPods", func() {
	const (
		myHost    = "test-host"
		myInvoc   = "test-invocation"
		foreignID = "foreign-invocation"
	)

	It("host-scoped purge only deletes pods matching own host + invocation-id", func() {
		m, cs := newManagerWithFakeClient()

		// Pod owned by this process.
		ownPod := makeLabeledPod("own-pod", map[string]string{
			"host":                     myHost,
			hipconsts.LabelOperationID: myInvoc,
		})
		// Pod owned by another concurrent process on the same host.
		foreignInvocPod := makeLabeledPod("foreign-invoc-pod", map[string]string{
			"host":                     myHost,
			hipconsts.LabelOperationID: foreignID,
		})
		// Pod owned by another host entirely.
		foreignHostPod := makeLabeledPod("foreign-host-pod", map[string]string{
			"host":                     "other-host",
			hipconsts.LabelOperationID: myInvoc, // same invoc id, different host
		})
		// Pod with no relevant labels (e.g. an unrelated workload).
		unrelatedPod := makeLabeledPod("unrelated-pod", map[string]string{
			"app": "other",
		})

		createPods(cs, ownPod, foreignInvocPod, foreignHostPod, unrelatedPod)

		Expect(m.DeleteHelmPods(cmdoptions.ExecOptions{}, cmdoptions.PurgeOptions{All: false})).
			To(Succeed())

		Expect(listPodNames(cs)).To(ConsistOf(
			"foreign-invoc-pod", "foreign-host-pod", "unrelated-pod"),
			"only own-pod should be removed; foreign-host, foreign-invocation, and unrelated pods must survive")
	})

	It("host-scoped purge with extra labels further narrows the selector", func() {
		m, cs := newManagerWithFakeClient()

		mineMatching := makeLabeledPod("mine-matching", map[string]string{
			"host":                     myHost,
			hipconsts.LabelOperationID: myInvoc,
			"test-id":                  "abc",
		})
		mineNotMatching := makeLabeledPod("mine-not-matching", map[string]string{
			"host":                     myHost,
			hipconsts.LabelOperationID: myInvoc,
			"test-id":                  "xyz",
		})
		createPods(cs, mineMatching, mineNotMatching)

		execOpts := cmdoptions.ExecOptions{Labels: map[string]string{"test-id": "abc"}}
		Expect(m.DeleteHelmPods(execOpts, cmdoptions.PurgeOptions{All: false})).To(Succeed())

		Expect(listPodNames(cs)).To(ConsistOf("mine-not-matching"),
			"only the pod with the matching extra label should be deleted")
	})

	It("--all purge deletes every pod in the namespace regardless of host/invocation", func() {
		m, cs := newManagerWithFakeClient()

		createPods(cs,
			makeLabeledPod("own", map[string]string{
				"host":                     myHost,
				hipconsts.LabelOperationID: myInvoc,
			}),
			makeLabeledPod("foreign-host", map[string]string{
				"host":                     "other-host",
				hipconsts.LabelOperationID: foreignID,
			}),
			makeLabeledPod("unrelated", map[string]string{"app": "other"}),
		)

		Expect(m.DeleteHelmPods(cmdoptions.ExecOptions{}, cmdoptions.PurgeOptions{All: true})).
			To(Succeed())

		Expect(listPodNames(cs)).To(BeEmpty(),
			"--all is namespace-wide; nothing should survive")
	})

	It("--all purge also sweeps PDBs whose pods were already removed", func() {
		m, cs := newManagerWithFakeClient()

		// Pre-create an orphaned PDB whose owning pod was already deleted
		// (e.g. by activeDeadlineSeconds firing or a previous cleanup pass).
		Expect(m.CreatePodDisruptionBudget(context.Background(), "orphan-op")).To(Succeed())

		Expect(m.DeleteHelmPods(cmdoptions.ExecOptions{}, cmdoptions.PurgeOptions{All: true})).
			To(Succeed())

		list, err := cs.PolicyV1().PodDisruptionBudgets(hipconsts.Namespace).List(
			context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(list.Items).To(BeEmpty(), "orphaned PDB must be cleaned by --all sweep")
	})

	It("host-scoped purge deletes the PDB for each deleted pod via operation-id label", func() {
		m, cs := newManagerWithFakeClient()

		// Create own pod + own PDB.
		createPods(cs, makeLabeledPod("own-pod", map[string]string{
			"host":                     myHost,
			hipconsts.LabelOperationID: myInvoc,
		}))
		Expect(m.CreatePodDisruptionBudget(context.Background(), myInvoc)).To(Succeed())

		// Create a foreign-invocation PDB that must survive.
		Expect(m.CreatePodDisruptionBudget(context.Background(), foreignID)).To(Succeed())

		Expect(m.DeleteHelmPods(cmdoptions.ExecOptions{}, cmdoptions.PurgeOptions{All: false})).
			To(Succeed())

		// Own pod gone.
		Expect(listPodNames(cs)).To(BeEmpty())

		// Own PDB gone; foreign PDB still present.
		pdbList, err := cs.PolicyV1().PodDisruptionBudgets(hipconsts.Namespace).List(
			context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(pdbList.Items).To(HaveLen(1))
		Expect(pdbList.Items[0].Labels[hipconsts.LabelOperationID]).To(Equal(foreignID),
			"foreign-invocation PDB must survive a host-scoped purge")
	})

	It("succeeds and is a no-op when no pods match", func() {
		m, _ := newManagerWithFakeClient()
		Expect(m.DeleteHelmPods(cmdoptions.ExecOptions{}, cmdoptions.PurgeOptions{All: false})).
			To(Succeed())
	})
})

var _ = Describe("DeleteKeptPods", func() {
	const myHost = "test-host"

	It("deletes only own-host pods with the kept=true label", func() {
		m, cs := newManagerWithFakeClient()

		ownKept := makeLabeledPod("own-kept", map[string]string{
			"host":              myHost,
			hipconsts.LabelKept: "true",
		})
		ownActive := makeLabeledPod("own-active", map[string]string{
			"host":                     myHost,
			hipconsts.LabelOperationID: "some-invoc", // not kept
		})
		foreignKept := makeLabeledPod("foreign-kept", map[string]string{
			"host":              "other-host",
			hipconsts.LabelKept: "true",
		})
		keptWithoutHost := makeLabeledPod("kept-without-host", map[string]string{
			hipconsts.LabelKept: "true",
		})

		createPods(cs, ownKept, ownActive, foreignKept, keptWithoutHost)

		Expect(m.DeleteKeptPods()).To(Succeed())

		Expect(listPodNames(cs)).To(ConsistOf(
			"own-active", "foreign-kept", "kept-without-host"),
			"only own-host kept pods should be deleted; foreign-host kept pods must survive")
	})

	It("succeeds and is a no-op when no kept pods exist on this host", func() {
		m, cs := newManagerWithFakeClient()
		createPods(cs, makeLabeledPod("own-not-kept", map[string]string{
			"host":                     myHost,
			hipconsts.LabelOperationID: "some-invoc",
		}))

		Expect(m.DeleteKeptPods()).To(Succeed())
		Expect(listPodNames(cs)).To(ConsistOf("own-not-kept"))
	})

	It("does not touch the kept pod's PDB if one happens to exist (kept pods do not own PDBs by default)", func() {
		// This locks in current behavior: DeleteKeptPods only deletes pods,
		// and only triggers PDB cleanup when the pod has an operation-id
		// label (which kept pods do NOT by design — see CreateHelmPod).
		m, cs := newManagerWithFakeClient()
		createPods(cs, makeLabeledPod("own-kept", map[string]string{
			"host":              myHost,
			hipconsts.LabelKept: "true",
			// NOTE: no operation-id label on a kept pod.
		}))

		Expect(m.DeleteKeptPods()).To(Succeed())
		Expect(listPodNames(cs)).To(BeEmpty())
	})
})
