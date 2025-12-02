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
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

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

// targetsSummary tells the aggregated result of the reconciliation.
// Useful for operators to quickly understand how many targets succeeded or failed
// without expanding the full TargetStatuses list.
type TargetsSummary struct {
	// Total number of target namespaces evaluated for this propagation in the last sync.
	Total int32 `json:"total,omitempty"`

	Created int32 `json:"created,omitempty"`

	Updated int32 `json:"updated,omitempty"`

	Deleted int32 `json:"deleted,omitempty"`

	Orphaned int32 `json:"orphaned,omitempty"`

	Failed int32 `json:"failed,omitempty"`
}

// TargetStatus represents the sync condition of a single target ConfigMap.
// Only include entries for failures, drift, or skipped targets to keep the
// status compact and readable.
type TargetStatus struct {
	// Namespace of the target ConfigMap.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`

	// Name of the target ConfigMap.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// State represents the controller's result for this specific target.
	// Common values:
	// - "Synced"   : successfully reconciled
	// - "Failed"   : update or creation error occurred
	// - "Drifted"  : manual modifications detected
	// - "Skipped"  : skipped due to CreateOnce or missing permissions
	// +kubebuilder:validation:MinLength=1
	State string `json:"state"`

	// Reason is a short machine-friendly code providing context for State.
	// Examples: "PermissionDenied", "DriftDetected", "NotFound".
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable explanation describing the issue or action taken.
	// Typically contains error details or drift description.
	// +optional
	Message string `json:"message,omitempty"`
}

// ConfigMapPropagationStatus defines the observed state of ConfigMapPropagation.
// Status reflects what the controller last observed and is used for debugging,
// reporting health, and showing aggregated results.
type ConfigMapPropagationStatus struct {
	// Conditions follow the standard Kubernetes conditions pattern.
	// Common types:
	// - Ready:    "True" when all targets are synced successfully
	// - Error:    "True" when an unrecoverable issue exists
	//
	// This list uses a map-style structure for efficient updates.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the metadata.generation that the controller
	// has last fully reconciled. Ensures users know the Status reflects
	// the latest Spec.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastSyncedAt is the timestamp of the most recent reconciliation attempt
	// (successful or failed). Useful for knowing controller liveness.
	LastSyncedAt metav1.Time `json:"lastSyncedAt,omitempty"`

	// TargetsSummary gives a compressed overview of how many targets succeeded
	// or failed during reconciliation.
	TargetsSummary TargetsSummary `json:"targetsSummary,omitempty"`

	// TargetStatuses contains detailed per-target records ONLY for targets that
	// failed, drifted, or were skipped. Healthy ones are omitted to avoid bloating
	// the CR in large clusters.
	// +optional
	TargetStatuses []TargetStatus `json:"targetStatuses,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="propagators.io/component=controller"
// +kubebuilder:resource:scope=Namespaced,categories={propagators}
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source.name`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:selectablefield:JSONPath=`.spec.source.name`
// +kubebuilder:selectablefield:JSONPath=`.spec.source.namespace`
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
