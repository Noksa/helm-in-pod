package hipns

import (
	"context"

	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Name = "helm-in-pod"

type Manager struct {
	clientSet *kubernetes.Clientset
	ctx       context.Context
}

func NewManager(clientSet *kubernetes.Clientset, ctx context.Context) *Manager {
	return &Manager{
		clientSet: clientSet,
		ctx:       ctx,
	}
}

func (m *Manager) PrepareNs() error {
	ns, err := m.clientSet.CoreV1().Namespaces().Get(m.ctx, Name, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if ns == nil || ns.Name == "" {
		log.Debugf("%v Creating '%v' ns", logz.LogHost(), Name)
		_, err = m.clientSet.CoreV1().Namespaces().Create(m.ctx, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: Name},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	sa, err := m.clientSet.CoreV1().ServiceAccounts(Name).Get(m.ctx, Name, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if sa == nil || sa.Name == "" {
		log.Debugf("%v Creating '%v' serviceaccount in '%v' ns", logz.LogHost(), Name, Name)
		_, err = m.clientSet.CoreV1().ServiceAccounts(Name).Create(m.ctx, &v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: Name},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return m.CreateClusterRoleBinding()
}

func (m *Manager) CreateClusterRoleBinding() error {
	crb, err := m.clientSet.RbacV1().ClusterRoleBindings().Get(m.ctx, Name, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if crb == nil || crb.Name == "" {
		log.Debugf("%v Creating '%v' clusterrolebinging in '%v' ns", logz.LogHost(), Name, Name)
		_, err = m.clientSet.RbacV1().ClusterRoleBindings().Create(m.ctx, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: Name},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: Name, Namespace: Name}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
		}, metav1.CreateOptions{})
	}
	return err
}

func (m *Manager) DeleteClusterRoleBinding() error {
	crb, err := m.clientSet.RbacV1().ClusterRoleBindings().Get(m.ctx, Name, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if crb != nil && crb.Name != "" {
		log.Debugf("%v Removing '%v' clusterrolebinging in '%v' ns", logz.LogHost(), Name, Name)
		err = m.clientSet.RbacV1().ClusterRoleBindings().Delete(m.ctx, Name, metav1.DeleteOptions{})
		return err
	}
	return nil
}
