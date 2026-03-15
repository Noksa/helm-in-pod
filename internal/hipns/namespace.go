package hipns

import (
	"context"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/noksa/helm-in-pod/internal/logz"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Name = "helm-in-pod"

type Manager struct {
	ctx context.Context
}

func NewManager(ctx context.Context) *Manager {
	return &Manager{ctx: ctx}
}

func (m *Manager) PrepareNs() error {
	cs := operatorkclient.DefaultClient().ClientSet()
	ns, err := cs.CoreV1().Namespaces().Get(m.ctx, Name, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if ns == nil || ns.Name == "" {
		logz.Host().Debug().Msgf("Creating '%v' ns", Name)
		_, err = cs.CoreV1().Namespaces().Create(m.ctx, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: Name},
		}, metav1.CreateOptions{})
		if err != nil && client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}
	sa, err := cs.CoreV1().ServiceAccounts(Name).Get(m.ctx, Name, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if sa == nil || sa.Name == "" {
		logz.Host().Debug().Msgf("Creating '%v' serviceaccount in '%v' ns", Name, Name)
		_, err = cs.CoreV1().ServiceAccounts(Name).Create(m.ctx, &v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: Name},
		}, metav1.CreateOptions{})
		if err != nil && client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}
	return m.CreateClusterRoleBinding()
}

func (m *Manager) CreateClusterRoleBinding() error {
	cs := operatorkclient.DefaultClient().ClientSet()
	crb, err := cs.RbacV1().ClusterRoleBindings().Get(m.ctx, Name, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if crb == nil || crb.Name == "" {
		logz.Host().Debug().Msgf("Creating '%v' clusterrolebinding in '%v' ns", Name, Name)
		_, err = cs.RbacV1().ClusterRoleBindings().Create(m.ctx, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: Name},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: Name, Namespace: Name}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
		}, metav1.CreateOptions{})
		if err != nil && client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) DeleteClusterRoleBinding() error {
	cs := operatorkclient.DefaultClient().ClientSet()
	crb, err := cs.RbacV1().ClusterRoleBindings().Get(m.ctx, Name, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if crb != nil && crb.Name != "" {
		logz.Host().Debug().Msgf("Removing '%v' clusterrolebinding in '%v' ns", Name, Name)
		err = cs.RbacV1().ClusterRoleBindings().Delete(m.ctx, Name, metav1.DeleteOptions{})
		return err
	}
	return nil
}
