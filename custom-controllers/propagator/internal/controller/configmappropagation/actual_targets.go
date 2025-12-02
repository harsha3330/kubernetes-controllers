package controller

import (
	"context"
	"fmt"

	syncv1alpha1 "github.com/harsha3330/kubernetes/custom-controllers/propagator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ConfigMapPropagationReconciler) getCurrentTargets(ctx context.Context, configmapPropagator *syncv1alpha1.ConfigMapPropagation) ([]*PropagatorTarget, error) {
	var configmapList corev1.ConfigMapList
	ownerLabelValue := fmt.Sprintf("%s.%s", configmapPropagator.Namespace, configmapPropagator.Name)
	if err := r.Client.List(ctx, &configmapList, client.MatchingLabels{
		OwnerLabelKey: ownerLabelValue,
	}); err != nil {
		return nil, err
	}
	targets := make([]*PropagatorTarget, 0)
	for _, configmap := range configmapList.Items {
		targets = append(targets, &PropagatorTarget{
			ConfigmapName: configmap.Name,
			Namespace:     configmap.Namespace,
		})
	}
	return targets, nil
}
