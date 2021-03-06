/*
Copyright 2022.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SandBoxSpec defines the desired state of SandBox
type SandBoxSpec struct {
	//+kubebuilder:validation:Required
	Name string `json:"name"`

	//+kubebuilder:validation:optional
	//+kubebuilder:validation:Enum=T1
	Type string `json:"type"`
}

// SandBoxStatus defines the observed state of SandBox
type SandBoxStatus struct {
	Name string `json:"name"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SandBox is the Schema for the sandboxes API
//+kubebbuilder:subresource:status
type SandBox struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SandBoxSpec   `json:"spec,omitempty"`
	Status SandBoxStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SandBoxList contains a list of SandBox
type SandBoxList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SandBox `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SandBox{}, &SandBoxList{})
}
