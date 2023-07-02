package internal

import (
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HelmPodNamespace struct {
}

func (h *HelmPodNamespace) PrepareNs() error {
	ns, err := clientSet.CoreV1().Namespaces().Get(ctx, HelmInPodNamespace, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if ns == nil || ns.Name == "" {
		log.Debugf("%v Creating '%v' ns", LogHost(), HelmInPodNamespace)
		_, err = clientSet.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: HelmInPodNamespace},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	sa, err := clientSet.CoreV1().ServiceAccounts(HelmInPodNamespace).Get(ctx, HelmInPodNamespace, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if sa == nil || sa.Name == "" {
		log.Debugf("%v Creating '%v' serviceaccount in '%v' ns", LogHost(), HelmInPodNamespace, HelmInPodNamespace)
		_, err = clientSet.CoreV1().ServiceAccounts(HelmInPodNamespace).Create(ctx, &v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: HelmInPodNamespace},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	err = h.DeleteClusterRoleBinding()
	if err != nil {
		return err
	}
	return h.CreateClusterRoleBinding()
}

func (h *HelmPodNamespace) CreateClusterRoleBinding() error {
	crb, err := clientSet.RbacV1().ClusterRoleBindings().Get(ctx, HelmInPodNamespace, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if crb == nil || crb.Name == "" {
		log.Debugf("%v Creating '%v' clusterrolebinging in '%v' ns", LogHost(), HelmInPodNamespace, HelmInPodNamespace)
		_, err = clientSet.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: HelmInPodNamespace},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: HelmInPodNamespace, Namespace: HelmInPodNamespace}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
		}, metav1.CreateOptions{})
	}
	return err
}

func (h *HelmPodNamespace) DeleteClusterRoleBinding() error {
	crb, err := clientSet.RbacV1().ClusterRoleBindings().Get(ctx, HelmInPodNamespace, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if crb != nil && crb.Name != "" {
		log.Debugf("%v Removing '%v' clusterrolebinging in '%v' ns", LogHost(), HelmInPodNamespace, HelmInPodNamespace)
		err = clientSet.RbacV1().ClusterRoleBindings().Delete(ctx, HelmInPodNamespace, metav1.DeleteOptions{})
	}
	return err
}
