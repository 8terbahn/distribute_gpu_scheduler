package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GPUPool represents a pool of homogeneous GPU workers.
// The operator tracks available capacity and assigns GPUWorkloads to the
// pool with the most free units that matches the requested GPU type.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=gpup
// +kubebuilder:printcolumn:name="GPU Type",type=string,JSONPath=`.spec.gpuType`
// +kubebuilder:printcolumn:name="Total",type=integer,JSONPath=`.spec.totalUnits`
// +kubebuilder:printcolumn:name="Available",type=integer,JSONPath=`.status.availableUnits`
// +kubebuilder:printcolumn:name="Allocated",type=integer,JSONPath=`.status.allocatedUnits`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type GPUPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPUPoolSpec   `json:"spec,omitempty"`
	Status GPUPoolStatus `json:"status,omitempty"`
}

// GPUPoolSpec defines the desired configuration of a GPU pool.
type GPUPoolSpec struct {
	// GPUType is the GPU model available in this pool.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=T4;V100;A100;H100
	GPUType string `json:"gpuType"`

	// TotalUnits is the total number of GPU units in this pool.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	TotalUnits int `json:"totalUnits"`

	// Labels are arbitrary key-value pairs propagated to provisioned resources.
	Labels map[string]string `json:"labels,omitempty"`
}

// GPUPoolStatus describes the observed capacity of the GPU pool.
type GPUPoolStatus struct {
	// AvailableUnits is the number of GPU units not yet allocated.
	AvailableUnits int `json:"availableUnits,omitempty"`

	// AllocatedUnits is the number of GPU units currently in use.
	AllocatedUnits int `json:"allocatedUnits,omitempty"`

	// Message is a human-readable summary of pool status.
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true

// GPUPoolList contains a list of GPUPool resources.
type GPUPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GPUPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GPUPool{}, &GPUPoolList{})
}
