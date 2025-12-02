package controller

import (
	"context"
	"fmt"
	"reflect"

	syncv1alpha1 "github.com/harsha3330/kubernetes/custom-controllers/propagator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ConfigMapPropagationReconciler) ensureConfigMap(ctx context.Context, cmp *syncv1alpha1.ConfigMapPropagation, t *PropagatorTarget) error {
	cm := &corev1.ConfigMap{}
	namespacedName := types.NamespacedName{Namespace: t.Namespace, Name: t.ConfigmapName}
	err := r.Get(ctx, namespacedName, cm)
	if err == nil {
		patched := false
		if cm.Labels == nil {
			cm.Labels = map[string]string{}
		}
		ownerLabelVal := fmt.Sprintf("%s.%s", cmp.Namespace, cmp.Name)
		if cm.Labels[OwnerLabelKey] != ownerLabelVal {
			cm.Labels[OwnerLabelKey] = ownerLabelVal
			patched = true
		}
		if cm.Labels[ManagedByLabelKey] != ManagedByLabelValue {
			cm.Labels[ManagedByLabelKey] = ManagedByLabelValue
			patched = true
		}
		if cm.Annotations == nil {
			cm.Annotations = map[string]string{}
		}
		if cm.Annotations[OwnerUIDAnnotation] != string(cmp.UID) {
			cm.Annotations[OwnerUIDAnnotation] = string(cmp.UID)
			patched = true
		}
		if patched {
			if err := r.Update(ctx, cm); err != nil {
				return fmt.Errorf("failed to patch labels/annotations on existing configmap: %w", err)
			}
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	srcName := cmp.Spec.Source.Name
	srcNS := cmp.Spec.Source.Namespace
	if srcNS == "" {
		srcNS = "default"
	}
	src := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: srcNS, Name: srcName}, src); err != nil {
		return fmt.Errorf("failed to get source ConfigMap %s/%s: %w", srcNS, srcName, err)
	}

	newCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.ConfigmapName,
			Namespace: t.Namespace,
			Labels: map[string]string{
				OwnerLabelKey:     fmt.Sprintf("%s.%s", cmp.Namespace, cmp.Name),
				ManagedByLabelKey: ManagedByLabelValue,
			},
			Annotations: map[string]string{
				OwnerUIDAnnotation: string(cmp.UID),
			},
		},
		Data:       src.Data,
		BinaryData: src.BinaryData,
	}

	if err := r.Create(ctx, newCM); err != nil {
		return fmt.Errorf("failed to create propagated configmap %s/%s: %w", t.Namespace, t.ConfigmapName, err)
	}
	return nil
}

func (r *ConfigMapPropagationReconciler) updateIfNeeded(ctx context.Context, cmp *syncv1alpha1.ConfigMapPropagation, t *PropagatorTarget) error {
	target := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: t.Namespace, Name: t.ConfigmapName}, target); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	srcNS := cmp.Spec.Source.Namespace
	if srcNS == "" {
		srcNS = "default"
	}
	src := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: srcNS, Name: cmp.Spec.Source.Name}, src); err != nil {
		return fmt.Errorf("failed to get source configmap for update: %w", err)
	}

	desiredData := map[string]string{}
	switch cmp.Spec.PropagationPolicy {
	case "Overwrite":
		desiredData = src.Data
	default:
		for k, v := range target.Data {
			desiredData[k] = v
		}
		for k, v := range src.Data {
			desiredData[k] = v
		}
	}

	if reflect.DeepEqual(target.Data, desiredData) {
		return nil
	}

	target.Data = desiredData
	if err := r.Update(ctx, target); err != nil {
		return fmt.Errorf("failed to update target configmap %s/%s: %w", t.Namespace, t.ConfigmapName, err)
	}
	return nil
}

func (r *ConfigMapPropagationReconciler) deleteConfigMap(ctx context.Context, ns, name string) error {
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return r.Delete(ctx, cm)
}

func (r *ConfigMapPropagationReconciler) orphanConfigMap(ctx context.Context, cmp *syncv1alpha1.ConfigMapPropagation, ns, name string) error {
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	changed := false
	if cm.Labels != nil {
		if lbl, ok := cm.Labels[OwnerLabelKey]; ok {
			expected := fmt.Sprintf("%s.%s", cmp.Namespace, cmp.Name)
			if lbl == expected {
				delete(cm.Labels, OwnerLabelKey)
				changed = true
			}
		}
	}
	if cm.Annotations != nil {
		if ann, ok := cm.Annotations[OwnerUIDAnnotation]; ok {
			if ann == string(cmp.UID) {
				delete(cm.Annotations, OwnerUIDAnnotation)
				changed = true
			}
		}
	}

	if changed {
		if err := r.Update(ctx, cm); err != nil {
			return fmt.Errorf("failed to patch configmap to orphan: %w", err)
		}
	}
	return nil
}
