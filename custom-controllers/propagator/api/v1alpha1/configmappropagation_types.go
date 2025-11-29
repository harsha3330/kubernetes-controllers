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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PropagationSource defines the Input Configmap for creating targets
type PropagationSource struct {
	// Name of the Configmap
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:Pattern:=^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the configmap
	// +kubebuilder:default="default"
	// +optional
	Namespace string `json:"namespace"`
}

type TargetRef struct {
	// Namespace where the propagated ConfigMap should be created/updated.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// Name of the target ConfigMap. If not provided, defaults to source name.
	// +optional
	Name string `json:"name,omitempty"`
}

// SyncMode defines how and when the Configmaps should be refreshed.
// +kubebuilder:validation:Enum=CreatedOnce;Periodic;OnChange
type SyncMode string

const (
	// SyncModeCreatedOnce creates the Configmap once and does not update it thereafter.
	SyncModeCreatedOnce SyncMode = "CreatedOnce"
	// SyncModePeriodic synchronizes the Configmap from the provider at regular intervals.
	SyncModePeriodic SyncMode = "Periodic"
	// SyncModeOnChange only synchronizes when the Configmap's metadata or spec changes.
	SyncModeOnChange SyncMode = "OnChange"
)

// Deletion Policy determines the state of target Configmaps when the source Configmap is deleted.
// +kubebuilder:validation:Enum=Delete;Orphan
type DeletionPolicy string

const (
	// PolicyDelete deletes the target configmap after the source is deleted
	DeletionPolicyDelete DeletionPolicy = "Delete"

	// PolicyDelete does not delete the target configmap after the source is deleted
	DeletionPolicyOrphan DeletionPolicy = "Orphan"
)

// PropagationPolicy determines how to pass in keys to the target Configmap.
// +kubebuilder:validation:Enum=Merge;Overwrite
type PropagationPolicy string

const (
	// PolicyDelete deletes the target configmap after the source is deleted
	PropagationPolicyMerge PropagationPolicy = "Merge"

	// PolicyDelete does not delete the target configmap after the source is deleted
	PropagationPolicyOverwrite PropagationPolicy = "Overwrite"
)

// ConfigMapPropagationSpec defines the desired state of ConfigMapPropagation
type ConfigMapPropagationSpec struct {
	// PropagationSource Defines the input for Propagation
	// Input the Configmap's name and namespace
	// If Namespace is not given , default namespace will be taken as input
	// +kubebuilder:validation:Required
	Source PropagationSource `json:"source"`

	// NamespaceSelector selects namespaces where the target ConfigMap
	// should be propagated.
	//
	// This reuses standard Kubernetes LabelSelector.
	// Example:
	//   namespaceSelector:
	//     matchLabels:
	//       team: backend
	//
	// Use Empty Object to match all namespaces example: namespaceSelector: {}
	// +kubebuilder:default={}
	// +optional
	NamespacSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// Explicit list of target namespaces/ConfigMaps.
	// +optional
	Targets []TargetRef `json:"targets,omitempty"`

	// DeletionPolicy tell what to do about the target configmap when the configmap is deleted
	// - Delete: Deletes the target ConfigMaps
	// - Orphan: Does not delete the target ConfigMaps
	// +kubebuilder:default="Delete"
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`

	// SyncMode determines how the Confimaps should be refreshed:
	// - CreatedOnce: Creates the Configmap only if it does not exist and does not update it thereafter
	// - Periodic: Synchronizes the Configmap from the external source at regular intervals specified by refreshInterval.
	//   No periodic updates occur if refreshInterval is 0.
	// - OnChange: Only synchronizes the Secret when the Configmap's metadata or specification changes
	// +kubebuilder:default="OnChange"
	// +optional
	SyncMode SyncMode `json:"syncMode,omitempty"`

	// SyncInterval determines how often to sync the target Configmap
	// Only Used when syncmode is periodic
	// +kubebuilder:default="5m"
	// +optional
	SyncInterval *metav1.Duration `json:"syncInterval,omitempty"`

	// GlobalCreateIfMissing determines whether to create a target Configmap when the configmap is not present
	// +kubebuilder:default=true
	// +kubebuilder:validation:Required
	CreateIfMissing bool `json:"createIfMissing"`

	// PropagationPolicy determines how the Confimaps should be refreshed:
	// - Overwrite: Keeps the target and source in sync and deletes the extra keys (Absolute Mirror)
	// - Merge: Add the keys without deleting the extra keys
	// +kubebuilder:default="Merge"
	// +optional
	PropagationPolicy PropagationPolicy `json:"propagationPolicy,omitempty"`

	// AllowSystem Namespaces determines if propagator needs to target System Namespace
	// +kubebuilder:default=true
	AllowSystemNamespaces bool `json:"allowSystemNamespaces,omitempty"`
}

// ConfigMapPropagationStatus defines the observed state of ConfigMapPropagation.
type ConfigMapPropagationStatus struct {
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ConfigMapPropagation is the Schema for the configmappropagations API
type ConfigMapPropagation struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ConfigMapPropagation
	// +required
	Spec ConfigMapPropagationSpec `json:"spec"`

	// status defines the observed state of ConfigMapPropagation
	// +optional
	Status ConfigMapPropagationStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ConfigMapPropagationList contains a list of ConfigMapPropagation
type ConfigMapPropagationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ConfigMapPropagation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigMapPropagation{}, &ConfigMapPropagationList{})
}
