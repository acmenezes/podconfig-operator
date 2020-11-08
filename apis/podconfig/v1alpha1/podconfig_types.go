/*


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

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VlanSpec type for Pods
type VlanSpec struct {
	ParentInterfaceName string `json:"parentInterfaceName,omitempty"`
	VlanID              int16  `json:"vlanID,omitempty"`
	BridgeName          string `json:"bridgeName,omitempty"`
}

// PodConfigSpec defines the desired state of PodConfig
type PodConfigSpec struct {
	// VLANs to be added to subinterfaces
	Vlans []VlanSpec `json:"vlans,omitempty"`
}

// PodConfigStatus defines the observed state of PodConfig
type PodConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PodConfig is the Schema for the podconfigs API
type PodConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodConfigSpec   `json:"spec"`
	Status PodConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PodConfigList contains a list of PodConfig
type PodConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodConfig{}, &PodConfigList{})
}
