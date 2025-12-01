/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	syncv1alpha1 "github.com/harsha3330/kubernetes/custom-controllers/propagator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type PropagatorTarget struct {
	ConfigmapName string
	Namespace     string
}

// ConfigMapPropagationReconciler reconciles a ConfigMapPropagation object
type ConfigMapPropagationReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=sync.propagators.io,resources=configmappropagations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sync.propagators.io,resources=configmappropagations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sync.propagators.io,resources=configmappropagations/finalizers,verbs=update
func (r *ConfigMapPropagationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("new sync request for configmap propagator", "configmap name", req.Name, "configmap ns", req.Namespace)
	log.Info("getting the configmap propagator resource with the client")

	var configmapPropagator syncv1alpha1.ConfigMapPropagation
	err := r.Client.Get(ctx, req.NamespacedName, &configmapPropagator)
	if err != nil {
		log.Error(err, "failed to get the configmap propagator using default client")
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("spec of configmap propagator", "cr spec", configmapPropagator.Spec)

	if !configmapPropagator.DeletionTimestamp.IsZero() {
		return r.HandleDelete(ctx, &configmapPropagator)
	}

	log.Info("Adding the Finalizer for configmap propagator")
	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&configmapPropagator, FinalizerName) {
		controllerutil.AddFinalizer(&configmapPropagator, FinalizerName)
		log.Info("Added the Finalizer for configmap propagator and updating using the client")
		if err := r.Update(ctx, &configmapPropagator); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ConfigMapPropagationReconciler) HandleDelete(ctx context.Context, configmapPropagator *syncv1alpha1.ConfigMapPropagation) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(configmapPropagator, FinalizerName) {
		return ctrl.Result{}, nil
	}

	if configmapPropagator.Spec.DeletionPolicy == "Orphan" {
		controllerutil.RemoveFinalizer(configmapPropagator, FinalizerName)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

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

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapPropagationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("configmap-propagator")
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncv1alpha1.ConfigMapPropagation{}).
		Named("configmappropagation").
		Complete(r)
}
