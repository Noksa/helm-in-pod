package hipns

import (
	"context"
	"fmt"
	"time"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	authorizationv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
	"github.com/noksa/helm-in-pod/internal/logz"
)

type Manager struct {
	ctx context.Context
}

func NewManager(ctx context.Context) *Manager {
	return &Manager{ctx: ctx}
}

// WithContext returns a copy of the manager using the provided context.
func (m *Manager) WithContext(ctx context.Context) *Manager {
	if ctx == nil {
		ctx = m.ctx
	}
	return &Manager{ctx: ctx}
}

func (m *Manager) PrepareNs() error {
	cs := operatorkclient.DefaultClient().ClientSet()
	ns, err := cs.CoreV1().Namespaces().Get(m.ctx, hipconsts.Namespace, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if ns == nil || ns.Name == "" {
		logz.Host().Debug().Msgf("Creating '%v' ns", hipconsts.Namespace)
		_, err = cs.CoreV1().Namespaces().Create(m.ctx, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: hipconsts.Namespace},
		}, metav1.CreateOptions{})
		if err != nil && client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}
	sa, err := cs.CoreV1().ServiceAccounts(hipconsts.Namespace).Get(m.ctx, hipconsts.Namespace, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if sa == nil || sa.Name == "" {
		logz.Host().Debug().Msgf("Creating '%v' serviceaccount in '%v' ns", hipconsts.Namespace, hipconsts.Namespace)
		_, err = cs.CoreV1().ServiceAccounts(hipconsts.Namespace).Create(m.ctx, &v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: hipconsts.Namespace},
		}, metav1.CreateOptions{})
		if err != nil && client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}
	return m.CreateClusterRoleBinding()
}

func (m *Manager) CreateClusterRoleBinding() error {
	cs := operatorkclient.DefaultClient().ClientSet()
	crb, err := cs.RbacV1().ClusterRoleBindings().Get(m.ctx, hipconsts.Namespace, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if crb == nil || crb.Name == "" {
		logz.Host().Debug().Msgf("Creating '%v' clusterrolebinding in '%v' ns", hipconsts.Namespace, hipconsts.Namespace)
		_, err = cs.RbacV1().ClusterRoleBindings().Create(m.ctx, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: hipconsts.Namespace},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: hipconsts.Namespace, Namespace: hipconsts.Namespace}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
		}, metav1.CreateOptions{})
		if err != nil && client.IgnoreAlreadyExists(err) != nil {
			return err
		}
		return m.waitForClusterRoleBindingEffective()
	}
	return nil
}

func (m *Manager) waitForClusterRoleBindingEffective() error {
	cs := operatorkclient.DefaultClient().ClientSet()
	saUser := "system:serviceaccount:" + hipconsts.Namespace + ":" + hipconsts.Namespace
	err := wait.PollUntilContextTimeout(m.ctx, time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		review := &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Verb:      "create",
					Resource:  "pods",
					Namespace: hipconsts.Namespace,
				},
				User: saUser,
			},
		}
		result, err := cs.AuthorizationV1().SubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
		if err != nil {
			return false, err
		}
		return result.Status.Allowed, nil
	})
	if wait.Interrupted(err) {
		return fmt.Errorf("timeout waiting for ClusterRoleBinding to become effective for %s", saUser)
	}
	return err
}

func (m *Manager) DeleteClusterRoleBinding() error {
	cs := operatorkclient.DefaultClient().ClientSet()
	crb, err := cs.RbacV1().ClusterRoleBindings().Get(m.ctx, hipconsts.Namespace, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if crb != nil && crb.Name != "" {
		logz.Host().Debug().Msgf("Removing '%v' clusterrolebinding in '%v' ns", hipconsts.Namespace, hipconsts.Namespace)
		err = cs.RbacV1().ClusterRoleBindings().Delete(m.ctx, hipconsts.Namespace, metav1.DeleteOptions{})
		return err
	}
	return nil
}
