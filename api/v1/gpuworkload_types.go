package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GPUWorkload is the Schema for GPU workload scheduling requests.
//
// Teams submit a GPUWorkload resource to declare a GPU compute job.
// The operator continuously reconciles the resource, assigning it to the
// best-fit GPUPool and tracking its lifecycle phase.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=gpuw
// +kubebuilder:printcolumn:name="GPU",type=string,JSONPath=`.spec.gpuRequirement`
// +kubebuilder:printcolumn:name="Priority",type=integer,JSONPath=`.spec.priority`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Worker",type=string,JSONPath=`.status.assignedWorker`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type GPUWorkload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPUWorkloadSpec   `json:"spec,omitempty"`
	Status GPUWorkloadStatus `json:"status,omitempty"`
}

// GPUWorkloadSpec defines the desired state of a GPU workload.
type GPUWorkloadSpec struct {
	// Workload is the type of compute job (e.g. llm_inference, gpu_benchmark).
	// +kubebuilder:validation:Required
	Workload string `json:"workload"`

	// GPURequirement specifies the GPU model required for this workload.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=T4;V100;A100;H100
	GPURequirement string `json:"gpuRequirement"`

	// Priority controls scheduling order. Higher value = scheduled first.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	Priority int `json:"priority"`

	// MaxRetries limits how many times a failed workload is re-queued.
	// +kubebuilder:default=3
	MaxRetries int `json:"maxRetries,omitempty"`
}

// GPUWorkloadStatus describes the observed state of a GPU workload.
type GPUWorkloadStatus struct {
	// Phase is the current lifecycle phase of the workload.
	// +kubebuilder:validation:Enum=Queued;Assigned;Running;Completed;Failed
	Phase string `json:"phase,omitempty"`

	// Message provides a human-readable explanation of the current phase.
	Message string `json:"message,omitempty"`

	// AssignedWorker is the name of the GPUPool this workload was dispatched to.
	AssignedWorker string `json:"assignedWorker,omitempty"`

	// StartTime records when the workload transitioned to Assigned.
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime records when the workload reached a terminal phase.
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}

// +kubebuilder:object:root=true

// GPUWorkloadList contains a list of GPUWorkload resources.
type GPUWorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GPUWorkload `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GPUWorkload{}, &GPUWorkloadList{})
}
