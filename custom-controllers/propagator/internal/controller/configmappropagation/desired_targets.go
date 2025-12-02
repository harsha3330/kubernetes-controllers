package controller

import (
	"context"

	syncv1alpha1 "github.com/harsha3330/kubernetes/custom-controllers/propagator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getDesiredTargets computes the desired targets from spec.targets and spec.namespaceSelector.
// It returns a deduplicated slice of PropagatorTarget.
func (r *ConfigMapPropagationReconciler) getDesiredTargets(ctx context.Context, configmapPropagator *syncv1alpha1.ConfigMapPropagation) ([]*PropagatorTarget, error) {
	targets := make([]*PropagatorTarget, 0)
	sourceName := configmapPropagator.Spec.Source.Name
	allowSystem := true
	allowSystem = configmapPropagator.Spec.AllowSystemNamespaces
	seen := make(map[string]struct{})

	// Explicit Target
	for _, t := range configmapPropagator.Spec.Targets {
		ns := t.Namespace
		if !allowSystem {
			if _, isSys := defaultSystemNamespaces[ns]; isSys {
				continue
			}
		}
		name := t.Name
		if name == "" {
			name = sourceName
		}
		key := ns + "/" + name
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, &PropagatorTarget{
			ConfigmapName: name,
			Namespace:     ns,
		})
	}

	nsSel := configmapPropagator.Spec.NamespaceSelector

	if nsSel != nil {
		sel, err := metav1.LabelSelectorAsSelector(nsSel)
		if err != nil {
			return nil, err
		}

		var nsList corev1.NamespaceList
		if err := r.List(ctx, &nsList, client.MatchingLabelsSelector{Selector: sel}); err != nil {
			return nil, err
		}

		for _, ns := range nsList.Items {
			if _, isSys := defaultSystemNamespaces[ns.Name]; !allowSystem && isSys {
				continue
			}
			key := ns.Name + "/" + sourceName
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			targets = append(targets, &PropagatorTarget{
				ConfigmapName: sourceName,
				Namespace:     ns.Name,
			})
		}
	}

	return targets, nil
}
