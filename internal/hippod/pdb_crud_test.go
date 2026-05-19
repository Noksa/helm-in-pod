package hippod

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clientruntime "k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

func newManagerWithFakeClient() (*Manager, *fake.Clientset) {
	cs := fake.NewClientset()
	installGenerateNameReactor(cs)
	installDeleteCollectionReactor(cs)
	dyn := dynamicfake.NewSimpleDynamicClient(clientruntime.Scheme)
	client := operatorkclient.NewClientFromClientSet(cs, dyn, nil)
	return &Manager{
		ctx:          context.Background(),
		myHostname:   "test-host",
		invocationID: "test-invocation",
		kclient:      client,
	}, cs
}

// installGenerateNameReactor mimics apiserver behavior: when an object is
// created with GenerateName set and Name unset, a unique Name is materialized.
// The default fake clientset does not do this, so concurrent creates collide
// on an empty Name.
func installGenerateNameReactor(cs *fake.Clientset) {
	var counter atomic.Uint64
	cs.PrependReactor("create", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		create, ok := action.(clienttesting.CreateAction)
		if !ok {
			return false, nil, nil
		}
		obj := create.GetObject()
		accessor, ok := obj.(metav1.Object)
		if !ok {
			return false, nil, nil
		}
		if accessor.GetName() == "" && accessor.GetGenerateName() != "" {
			accessor.SetName(fmt.Sprintf("%s%d", accessor.GetGenerateName(), counter.Add(1)))
		}
		return false, nil, nil
	})
}

// installDeleteCollectionReactor mimics apiserver behavior for DeleteCollection
// by walking the tracker and removing PDBs that match the LabelSelector from
// the action. The default fake clientset ignores LabelSelector on
// DeleteCollection. The tracker is used directly because calling cs.X() from
// inside a reactor deadlocks on the same internal mutex.
func installDeleteCollectionReactor(cs *fake.Clientset) {
	cs.PrependReactor("delete-collection", "poddisruptionbudgets", func(action clienttesting.Action) (bool, runtime.Object, error) {
		dca, ok := action.(clienttesting.DeleteCollectionAction)
		if !ok {
			return false, nil, nil
		}
		gvr := dca.GetResource()
		ns := dca.GetNamespace()
		gvk := schema.GroupVersionKind{Group: "policy", Version: "v1", Kind: "PodDisruptionBudget"}
		listObj, err := cs.Tracker().List(gvr, gvk, ns)
		if err != nil {
			return true, nil, err
		}
		list, ok := listObj.(*policyv1.PodDisruptionBudgetList)
		if !ok {
			return false, nil, nil
		}
		selector := dca.GetListRestrictions().Labels
		if selector == nil {
			selector = labels.Everything()
		}
		for i := range list.Items {
			item := &list.Items[i]
			if !selector.Matches(labels.Set(item.Labels)) {
				continue
			}
			if delErr := cs.Tracker().Delete(gvr, ns, item.Name); delErr != nil {
				return true, nil, delErr
			}
		}
		return true, nil, nil
	})
}

var _ = Describe("PDB CRUD", func() {
	Describe("CreatePodDisruptionBudget", func() {
		It("creates a PDB with operation-id label, generate-name prefix, and minAvailable=1", func() {
			m, cs := newManagerWithFakeClient()
			Expect(m.CreatePodDisruptionBudget(context.Background(), "op-123")).To(Succeed())

			list, err := cs.PolicyV1().PodDisruptionBudgets(hipconsts.Namespace).List(
				context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))

			pdb := list.Items[0]
			Expect(pdb.Labels[hipconsts.LabelOperationID]).To(Equal("op-123"))
			Expect(pdb.GenerateName).To(Equal(hipconsts.Namespace + "-pdb-"))
			Expect(pdb.Namespace).To(Equal(hipconsts.Namespace))

			expectedMin := intstr.FromInt(1)
			Expect(pdb.Spec.MinAvailable).NotTo(BeNil())
			Expect(*pdb.Spec.MinAvailable).To(Equal(expectedMin))

			Expect(pdb.Spec.Selector).NotTo(BeNil())
			Expect(pdb.Spec.Selector.MatchLabels).To(HaveKeyWithValue(hipconsts.LabelOperationID, "op-123"))
		})

		It("creates independent PDBs for distinct operation IDs", func() {
			m, cs := newManagerWithFakeClient()
			Expect(m.CreatePodDisruptionBudget(context.Background(), "op-a")).To(Succeed())
			Expect(m.CreatePodDisruptionBudget(context.Background(), "op-b")).To(Succeed())

			list, err := cs.PolicyV1().PodDisruptionBudgets(hipconsts.Namespace).List(
				context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(2))

			ids := []string{
				list.Items[0].Labels[hipconsts.LabelOperationID],
				list.Items[1].Labels[hipconsts.LabelOperationID],
			}
			Expect(ids).To(ConsistOf("op-a", "op-b"))
		})

		It("respects a custom namespace from hipconsts.Namespace", func() {
			original := hipconsts.Namespace
			hipconsts.Namespace = "alt-ns"
			DeferCleanup(func() { hipconsts.Namespace = original })

			m, cs := newManagerWithFakeClient()
			Expect(m.CreatePodDisruptionBudget(context.Background(), "op-1")).To(Succeed())

			list, err := cs.PolicyV1().PodDisruptionBudgets("alt-ns").List(
				context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items[0].GenerateName).To(Equal("alt-ns-pdb-"))
		})
	})

	Describe("DeletePodDisruptionBudgets", func() {
		It("deletes only PDBs matching the operation ID label", func() {
			m, _ := newManagerWithFakeClient()
			Expect(m.CreatePodDisruptionBudget(context.Background(), "keep")).To(Succeed())
			Expect(m.CreatePodDisruptionBudget(context.Background(), "delete-me")).To(Succeed())

			Expect(m.DeletePodDisruptionBudgets(context.Background(), "delete-me")).To(Succeed())

			cs := m.client().ClientSet()
			list, err := cs.PolicyV1().PodDisruptionBudgets(hipconsts.Namespace).List(
				context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items[0].Labels[hipconsts.LabelOperationID]).To(Equal("keep"))
		})

		It("succeeds and is a no-op when no PDBs match", func() {
			m, _ := newManagerWithFakeClient()
			Expect(m.CreatePodDisruptionBudget(context.Background(), "op-only")).To(Succeed())

			Expect(m.DeletePodDisruptionBudgets(context.Background(), "nonexistent")).To(Succeed())

			cs := m.client().ClientSet()
			list, err := cs.PolicyV1().PodDisruptionBudgets(hipconsts.Namespace).List(
				context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))
		})

		It("succeeds when the namespace has no PDBs", func() {
			m, _ := newManagerWithFakeClient()
			Expect(m.DeletePodDisruptionBudgets(context.Background(), "any")).To(Succeed())
		})
	})

	Describe("deleteAllPodDisruptionBudgets (purge --all sweep)", func() {
		It("deletes every PDB in the plugin namespace regardless of label", func() {
			m, cs := newManagerWithFakeClient()
			Expect(m.CreatePodDisruptionBudget(context.Background(), "op-1")).To(Succeed())
			Expect(m.CreatePodDisruptionBudget(context.Background(), "op-2")).To(Succeed())

			_, err := cs.PolicyV1().PodDisruptionBudgets(hipconsts.Namespace).Create(
				context.Background(),
				&policyv1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "orphan",
						Namespace: hipconsts.Namespace,
					},
				},
				metav1.CreateOptions{},
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(m.deleteAllPodDisruptionBudgets()).To(Succeed())

			list, err := cs.PolicyV1().PodDisruptionBudgets(hipconsts.Namespace).List(
				context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(BeEmpty())
		})

		It("succeeds when the namespace has no PDBs", func() {
			m, _ := newManagerWithFakeClient()
			Expect(m.deleteAllPodDisruptionBudgets()).To(Succeed())
		})
	})
})
