// package v1 contains API Schema definitions for the scheduling.gpu-platform.io API group.
// +kubebuilder:object:generate=true
// +groupName=scheduling.gpu-platform.io
package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion  = schema.GroupVersion{Group: "scheduling.gpu-platform.io", Version: "v1alpha1"}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme   = SchemeBuilder.AddToScheme
)
