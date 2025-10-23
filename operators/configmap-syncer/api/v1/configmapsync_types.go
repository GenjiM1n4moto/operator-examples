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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ConfigMapSyncSpec defines the desired state of ConfigMapSync.
type ConfigMapSyncSpec struct {
	// SourceConfigMap specifies the source ConfigMap to sync
	SourceConfigMap ConfigMapReference `json:"sourceConfigMap"`

	// TargetNamespaces specifies the list of target namespaces to sync to
	// +optional
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`

	// Selector is an optional label selector to filter target namespaces
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// ConfigMapReference contains the reference to a ConfigMap
type ConfigMapReference struct {
	// Name is the name of the ConfigMap
	Name string `json:"name"`

	// Namespace is the namespace of the ConfigMap
	Namespace string `json:"namespace"`
}

// ConfigMapSyncStatus defines the observed state of ConfigMapSync.
type ConfigMapSyncStatus struct {
	// SyncedNamespaces contains the list of namespaces where the ConfigMap has been synced
	SyncedNamespaces []string `json:"syncedNamespaces,omitempty"`

	// LastSyncTime represents the last time the sync was performed
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Conditions represent the latest available observations of the ConfigMapSync's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ConfigMapSync is the Schema for the configmapsyncs API.
type ConfigMapSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigMapSyncSpec   `json:"spec,omitempty"`
	Status ConfigMapSyncStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigMapSyncList contains a list of ConfigMapSync.
type ConfigMapSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigMapSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigMapSync{}, &ConfigMapSyncList{})
}
