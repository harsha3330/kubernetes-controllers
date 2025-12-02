package controller

import (
	"context"
	"fmt"
	"strings"

	syncv1alpha1 "github.com/harsha3330/kubernetes/custom-controllers/propagator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ConfigMapPropagationReconciler) HandleDelete(ctx context.Context, configmapPropagator *syncv1alpha1.ConfigMapPropagation) error {
	if !controllerutil.ContainsFinalizer(configmapPropagator, FinalizerName) {
		return nil
	}

	targets, err := r.getCurrentTargets(ctx, configmapPropagator)
	if err != nil {
		return err
	}

	failedTargets := make([]*PropagatorTarget, 0)

	for _, target := range targets {
		var err error
		switch configmapPropagator.Spec.DeletionPolicy {
		case "Delete":
			err = r.deleteConfigMap(ctx, target.Namespace, target.ConfigmapName)
		case "Orphan":
			err = r.orphanConfigMap(ctx, configmapPropagator, target.Namespace, target.ConfigmapName)
		}

		if err != nil {
			failedTargets = append(failedTargets, target)
		}
	}

	if len(failedTargets) > 0 {
		parts := make([]string, 0, len(failedTargets))
		for _, t := range failedTargets {
			parts = append(parts, fmt.Sprintf("%s/%s", t.Namespace, t.ConfigmapName))
		}
		errMsg := fmt.Errorf("%v: %s", ErrDeletingTargets, strings.Join(parts, ","))
		return errMsg
	}

	controllerutil.RemoveFinalizer(configmapPropagator, FinalizerName)
	if err := r.Update(ctx, configmapPropagator); err != nil {
		return err
	}

	return nil
}
