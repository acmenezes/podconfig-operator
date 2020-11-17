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

// Link type for new Pod interfaces
type Link struct {
	Name     string `json:"name,omitempty"`
	LinkType string `json:"linkType,omitemtpy"` // temporarily used for veth pair
	Parent   string `json:"parent,omitemtpy"`   // name for the parent interface
	Master   string `json:"master,omitempty"`   // name for the master bridge
	CIDR     string `json:"cidr,omitempty"`

	// For use with the netlink package  may access all types on the ip stack
	// Index        int                     `json:"index,omitempty"`
	// MTU          int                     `json:"mtu,omitempty"`
	// TxQLen       int                     `json:"txqlen,omitempty"` // Transmit Queue Length
	// HardwareAddr net.HardwareAddr        `json:"hardwareAddr,omitempty"`
	// Flags        net.Flags               `json:"flags,omitempty"`
	// RawFlags     uint32                  `json:"rawFlags,omitempty"`
	// ParentIndex  int                     `json:"parentIndex,omitempty"` // index of the parent link device
	// MasterIndex  int                     `json:"masterIndex,omitempty"` // must be the index of a bridge
	// Alias        string                  `json:"alias,omitempty"`
	// Statistics   *netlink.LinkStatistics `json:"statistics,omitempty"`
	// Promisc      int                     `json:"promisc,omitempty"`
	// Xdp          *netlink.LinkXdp        `json:"xdp,omitempty"`
	// EncapType    string                  `json:"encapType,omitempty"`
	// Protinfo     *netlink.Protinfo       `json:"protinfo,omitempty"`
	// OperState    netlink.LinkOperState   `json:"operState,omitempty"`
	// NumTxQueues  int                     `json:"numTxQueues,omitempty"`
	// NumRxQueues  int                     `json:"numRxQueues,omitempty"`
	// GSOMaxSize   uint32                  `json:"gsoMaxSize,omitempty"`
	// GSOMaxSegs   uint32                  `json:"gsoMaxSegs,omitempty"`
	// Vfs          []netlink.VfInfo        `json:"vfs,omitempty"` // virtual functions available on link
	// Group        uint32                  `json:"group,omitempty"`
	// Slave        netlink.LinkSlave       `json:"slave,omitempty"`
}

// SampleResource for testing with pods
type SampleResource struct {
	Create bool   `json:"create,omitempty"`
	Name   string `json:"name,omitempty"`
}

// PodConfigSpec defines the desired state of PodConfig
type PodConfigSpec struct {
	// Flag to enable sample deployment
	SampleDeployment SampleResource `json:"sampleDeployment,omitempty"`

	// List of new interfaces to configure on Pod
	NetworkAttachments []Link `json:"networkAttachments,omitempty"`

	// VLANs to be added to subinterfaces
	Vlans []VlanSpec `json:"vlans,omitempty"`
}

// PodConfigPhase type for status
type PodConfigPhase string

// Pod config const values
const (
	PodConfigUnSet       PodConfigPhase = "unset"
	PodConfigConfiguring PodConfigPhase = "configuring"
	PodConfigConfigured  PodConfigPhase = "configured"
)

// PodConfiguration for status
type PodConfiguration struct {
	PodName    string   `json:"podName,omitempty"`
	ConfigList []string `json:"configList,omitemtpy"`
}

// PodConfigStatus defines the observed state of PodConfig
type PodConfigStatus struct {
	// Phase is unset, configuring or configured
	Phase             PodConfigPhase     `json:"phase,omitempty"`
	PodConfigurations []PodConfiguration `json:"podConfigurations,omitemtpy"`
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
