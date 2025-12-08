package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	syncv1alpha1 "github.com/harsha3330/kubernetes/custom-controllers/propagator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ConfigMapPropagationReconciler) SyncTargets(ctx context.Context, configmapPropagator *syncv1alpha1.ConfigMapPropagation) (ctrl.Result, error) {
	desired, err := r.getDesiredTargets(ctx, configmapPropagator)
	if err != nil {
		r.Recorder.Eventf(configmapPropagator, corev1.EventTypeWarning, "Compute Desired Failed", "failed to compute desired targets: %v", err)
		return ctrl.Result{}, err
	}

	current, err := r.getCurrentTargets(ctx, configmapPropagator)
	if err != nil {
		r.Recorder.Eventf(configmapPropagator, corev1.EventTypeWarning, "List Children Failed", "failed to list managed ConfigMaps: %v", err)
		return ctrl.Result{}, err
	}

	desiredMap := make(map[string]*PropagatorTarget)
	for _, target := range desired {
		key := target.Namespace + "/" + target.ConfigmapName
		desiredMap[key] = target
	}

	currentMap := make(map[string]*PropagatorTarget)
	for _, target := range current {
		key := target.Namespace + "/" + target.ConfigmapName
		currentMap[key] = target
	}

	toCreate := make([]*PropagatorTarget, 0)
	toUpdate := make([]*PropagatorTarget, 0)
	toDelete := make([]*PropagatorTarget, 0)

	// Loop the DesiredMap check if exists in currentMap
	// If it exists , update
	// If not Create
	for key, target := range desiredMap {
		if _, exists := currentMap[key]; !exists {
			toCreate = append(toCreate, target)
		} else {
			toUpdate = append(toUpdate, target)
		}
	}

	// Loop the CurrentMap check if target is not in desired
	// If not delete it
	for key, target := range currentMap {
		if _, exists := desiredMap[key]; !exists {
			toDelete = append(toDelete, target)
		}
	}

	var targetSummary syncv1alpha1.TargetsSummary = syncv1alpha1.TargetsSummary{}
	var targetStatuses []syncv1alpha1.TargetStatus = make([]syncv1alpha1.TargetStatus, 0)

	for _, t := range toCreate {
		if err := r.ensureConfigMap(ctx, configmapPropagator, t); err != nil {
			targetSummary.Failed += 1
			targetStatuses = append(targetStatuses, syncv1alpha1.TargetStatus{
				Namespace: t.Namespace,
				Name:      t.ConfigmapName,
				State:     "Failed",
				Reason:    fmt.Sprintf("%v", err),
				Message:   "Failed to Ensure the configmap",
			})
			r.Recorder.Eventf(configmapPropagator, corev1.EventTypeNormal, "CreatedFailed", "%s/%s creation failed : %v", t.Namespace, t.ConfigmapName, err)
		} else {
			targetSummary.Created += 1
		}
		targetSummary.Total += 1
	}

	for _, t := range toUpdate {
		if err := r.updateIfNeeded(ctx, configmapPropagator, t); err != nil {
			r.Recorder.Eventf(configmapPropagator, corev1.EventTypeWarning, "UpdateFailed", " %s/%s update failed: %v", t.Namespace, t.ConfigmapName, err)
			targetStatuses = append(targetStatuses, syncv1alpha1.TargetStatus{
				Namespace: t.Namespace,
				Name:      t.ConfigmapName,
				State:     "Failed",
				Reason:    fmt.Sprintf("%v", err),
				Message:   "Failed to update the configmap",
			})
		} else {
			targetSummary.Updated += 1
		}
		targetSummary.Total += 1
	}

	for _, t := range toDelete {
		switch configmapPropagator.Spec.DeletionPolicy {
		case "Delete":
			if err := r.deleteConfigMap(ctx, t.Namespace, t.ConfigmapName); err != nil {
				r.Recorder.Eventf(configmapPropagator, corev1.EventTypeWarning, "DeleteFailed", " %s/%s delete failed: %v", t.Namespace, t.ConfigmapName, err)
				targetSummary.Failed += 1
			} else {
				targetSummary.Deleted += 1
			}
			targetSummary.Total += 1
			r.Recorder.Eventf(configmapPropagator, corev1.EventTypeNormal, "DeletedTarget", "deleted propagated ConfigMap %s/%s", t.Namespace, t.ConfigmapName)
		case "Orphan":
			if err := r.orphanConfigMap(ctx, configmapPropagator, t.Namespace, t.ConfigmapName); err != nil {
				r.Recorder.Eventf(configmapPropagator, corev1.EventTypeWarning, "OrphanFailed", " %s/%s orphan failed: %v", t.Namespace, t.ConfigmapName, err)
				targetSummary.Failed += 1
			} else {
				targetSummary.Orphaned += 1
			}
			targetSummary.Total += 1
			r.Recorder.Eventf(configmapPropagator, corev1.EventTypeNormal, "OrphanedTarget", "Orphaned propagated ConfigMap %s/%s", t.Namespace, t.ConfigmapName)
		}
	}

	updateCmp := configmapPropagator.DeepCopy()

	updateCmp.Status.TargetsSummary = targetSummary
	updateCmp.Status.TargetStatuses = targetStatuses
	updateCmp.Status.LastSyncedAt = metav1.NewTime(time.Now())
	if targetSummary.Failed > 0 {
		failedParts := make([]string, 0, len(targetStatuses))
		for _, t := range targetStatuses {
			failedParts = append(failedParts, fmt.Sprintf("%s/%s", t.Namespace, t.Name))
		}
		meta.SetStatusCondition(&updateCmp.Status.Conditions, metav1.Condition{
			Type:    "UnReady",
			Status:  metav1.ConditionFalse,
			Reason:  "SyncFailed",
			Message: fmt.Sprintf("Sync Failed for: %s", strings.Join(failedParts, ",")),
		})
	} else {
		updateCmp.Status.SyncedResourceVersion = configmapPropagator.ResourceVersion
		meta.SetStatusCondition(&updateCmp.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  "Synced",
			Message: "All Objects have been synced",
		})
	}

	if !equality.Semantic.DeepEqual(configmapPropagator.Status, updateCmp.Status) {
		if err := r.Status().Patch(ctx, updateCmp, client.MergeFrom(configmapPropagator)); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update the status of configmappropagator")
		}
	}

	if targetSummary.Failed > 0 {
		return ctrl.Result{}, fmt.Errorf("failed to sync the targets")
	}

	return ctrl.Result{}, nil
}
