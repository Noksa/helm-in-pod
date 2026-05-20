package hipns

import (
	"context"
	"testing"
	"time"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	clientruntime "k8s.io/client-go/kubernetes/scheme"
	ktesting "k8s.io/client-go/testing"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

func TestHipns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hipns Suite")
}

func newTestManager() (*Manager, *fakek8s.Clientset) {
	cs := fakek8s.NewClientset()
	// Ensure SAR checks in waitForClusterRoleBindingEffective always succeed in unit tests
	cs.PrependReactor("create", "subjectaccessreviews", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &authorizationv1.SubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
		}, nil
	})
	dyn := dynamicfake.NewSimpleDynamicClient(clientruntime.Scheme)
	client := operatorkclient.NewClientFromClientSet(cs, dyn, nil)
	return &Manager{ctx: context.Background(), kclient: client}, cs
}

var _ = Describe("hipns.Manager", func() {
	Describe("NewManager", func() {
		It("returns a manager with the provided context and no client override", func() {
			ctx := context.Background()
			m := NewManager(ctx)
			Expect(m).NotTo(BeNil())
			Expect(m.ctx).To(Equal(ctx))
			Expect(m.kclient).To(BeNil())
		})
	})

	Describe("WithContext", func() {
		It("preserves the original context when nil is passed", func() {
			orig := context.Background()
			m := NewManager(orig)
			var nilCtx context.Context
			swapped := m.WithContext(nilCtx)
			Expect(swapped.ctx).To(Equal(orig))
		})

		It("swaps to the provided context when non-nil", func() {
			m := NewManager(context.Background())
			newCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			swapped := m.WithContext(newCtx)
			Expect(swapped.ctx).To(Equal(newCtx))
		})

		It("propagates the injected client across WithContext calls", func() {
			m, _ := newTestManager()
			original := m.kclient
			swapped := m.WithContext(context.Background())
			Expect(swapped.kclient).To(BeIdenticalTo(original))
		})
	})

	Describe("PrepareNs", func() {
		It("creates the namespace, service account, and cluster role binding when none exist", func() {
			m, cs := newTestManager()
			Expect(m.PrepareNs()).To(Succeed())

			ns, err := cs.CoreV1().Namespaces().Get(context.Background(), hipconsts.Namespace, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Name).To(Equal(hipconsts.Namespace))

			sa, err := cs.CoreV1().ServiceAccounts(hipconsts.Namespace).Get(context.Background(), hipconsts.Namespace, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(sa.Name).To(Equal(hipconsts.Namespace))

			crb, err := cs.RbacV1().ClusterRoleBindings().Get(context.Background(), hipconsts.Namespace, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(crb.RoleRef.Name).To(Equal("cluster-admin"))
			Expect(crb.Subjects).To(HaveLen(1))
			Expect(crb.Subjects[0]).To(Equal(rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      hipconsts.Namespace,
				Namespace: hipconsts.Namespace,
			}))
		})

		It("is idempotent when namespace already exists", func() {
			m, cs := newTestManager()
			_, err := cs.CoreV1().Namespaces().Create(context.Background(),
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: hipconsts.Namespace}},
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(m.PrepareNs()).To(Succeed())

			sa, err := cs.CoreV1().ServiceAccounts(hipconsts.Namespace).Get(context.Background(), hipconsts.Namespace, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(sa.Name).To(Equal(hipconsts.Namespace))
		})

		It("respects a custom namespace from hipconsts.Namespace", func() {
			original := hipconsts.Namespace
			hipconsts.Namespace = "custom-ns"
			DeferCleanup(func() { hipconsts.Namespace = original })

			m, cs := newTestManager()
			Expect(m.PrepareNs()).To(Succeed())

			ns, err := cs.CoreV1().Namespaces().Get(context.Background(), "custom-ns", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ns.Name).To(Equal("custom-ns"))

			crb, err := cs.RbacV1().ClusterRoleBindings().Get(context.Background(), "custom-ns", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(crb.Subjects[0].Namespace).To(Equal("custom-ns"))
		})

		It("waits for ClusterRoleBinding to become effective", func() {
			m, cs := newTestManager()

			Expect(m.PrepareNs()).To(Succeed())

			Eventually(func(g Gomega) {
				review := &authorizationv1.SubjectAccessReview{
					Spec: authorizationv1.SubjectAccessReviewSpec{
						ResourceAttributes: &authorizationv1.ResourceAttributes{
							Verb:      "create",
							Resource:  "pods",
							Namespace: hipconsts.Namespace,
						},
						User: "system:serviceaccount:" + hipconsts.Namespace + ":" + hipconsts.Namespace,
					},
				}
				result, err := cs.AuthorizationV1().SubjectAccessReviews().Create(context.Background(), review, metav1.CreateOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result.Status.Allowed).To(BeTrue())
			}).WithTimeout(30 * time.Second).Should(Succeed())
		})
	})

	Describe("CreateClusterRoleBinding", func() {
		It("creates the binding when none exists", func() {
			m, cs := newTestManager()
			Expect(m.CreateClusterRoleBinding()).To(Succeed())

			crb, err := cs.RbacV1().ClusterRoleBindings().Get(context.Background(), hipconsts.Namespace, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(crb.RoleRef.Kind).To(Equal("ClusterRole"))
			Expect(crb.RoleRef.Name).To(Equal("cluster-admin"))
		})

		It("is a no-op when the binding already exists", func() {
			m, cs := newTestManager()
			Expect(m.CreateClusterRoleBinding()).To(Succeed())
			Expect(m.CreateClusterRoleBinding()).To(Succeed())

			list, err := cs.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))
		})
	})

	Describe("DeleteClusterRoleBinding", func() {
		It("removes an existing binding", func() {
			m, cs := newTestManager()
			Expect(m.CreateClusterRoleBinding()).To(Succeed())

			Expect(m.DeleteClusterRoleBinding()).To(Succeed())

			_, err := cs.RbacV1().ClusterRoleBindings().Get(context.Background(), hipconsts.Namespace, metav1.GetOptions{})
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		It("is a no-op when the binding does not exist", func() {
			m, _ := newTestManager()
			Expect(m.DeleteClusterRoleBinding()).To(Succeed())
		})
	})
})
