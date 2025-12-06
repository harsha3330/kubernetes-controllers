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
	"errors"
	"time"

	syncv1alpha1 "github.com/harsha3330/kubernetes/custom-controllers/propagator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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

	if configmapPropagator.Status.ObservedGeneration == configmapPropagator.Generation {
		return ctrl.Result{}, err
	}

	log.Info("spec of configmap propagator", "cr spec", configmapPropagator.Spec)

	// Checking for Deletion Timestamp and deleting the cr if present
	if !configmapPropagator.DeletionTimestamp.IsZero() {
		err := r.HandleDelete(ctx, &configmapPropagator)
		r.Recorder.Eventf(&configmapPropagator, corev1.EventTypeWarning, "Delete Failed", "%v", err)
		if err != nil {
			if errors.Is(err, ErrDeletingTargets) {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
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

	// Check for intial ConfigMap
	var sourceConfig v1.ConfigMap
	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      configmapPropagator.Spec.Source.Name,
		Namespace: configmapPropagator.Spec.Source.Namespace,
	}, &sourceConfig)

	if err != nil {
		r.Recorder.Eventf(&configmapPropagator, corev1.EventTypeWarning, "SourceConfigMap Not Found", "%v", err)
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, err
	}

	return r.SyncTargets(ctx, &configmapPropagator)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapPropagationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("configmap-propagator")
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncv1alpha1.ConfigMapPropagation{}).
		Named("configmappropagation").
		Complete(r)
}
