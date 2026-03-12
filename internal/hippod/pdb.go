package hippod

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	log "github.com/sirupsen/logrus"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// GenerateOperationID generates a unique UUID for a helm-in-pod operation
func GenerateOperationID() string {
	return uuid.New().String()
}

// CreatePodDisruptionBudget creates a PDB for the given pod with the operation ID
func (m *Manager) CreatePodDisruptionBudget(ctx context.Context, operationID string) error {
	minAvailable := intstr.FromInt(1)
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-pdb-", Namespace),
			Namespace:    Namespace,
			Labels: map[string]string{
				hipconsts.LabelOperationID: operationID,
			},
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					hipconsts.LabelOperationID: operationID,
				},
			},
		},
	}

	_, err := m.clientSet.PolicyV1().PodDisruptionBudgets(Namespace).Create(ctx, pdb, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PodDisruptionBudget: %w", err)
	}

	log.Debugf("Created PodDisruptionBudget for operation %s", operationID)
	return nil
}

// DeletePodDisruptionBudgets deletes all PDBs matching the given operation ID
func (m *Manager) DeletePodDisruptionBudgets(ctx context.Context, operationID string) error {
	labelSelector := fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, operationID)

	err := m.clientSet.PolicyV1().PodDisruptionBudgets(Namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete PodDisruptionBudgets: %w", err)
	}

	log.Debugf("Deleted PodDisruptionBudgets for operation %s", operationID)
	return nil
}
