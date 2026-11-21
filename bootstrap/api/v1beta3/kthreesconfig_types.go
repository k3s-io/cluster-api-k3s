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

package v1beta3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KThreesConfigSpec defines the desired state of KThreesConfig.
type KThreesConfigSpec struct {
	// Files specifies extra files to be passed to user_data upon creation.
	// +optional
	Files []File `json:"files,omitempty"`

	// PreK3sCommands specifies extra commands to run before k3s setup runs
	// +optional
	PreK3sCommands []string `json:"preK3sCommands,omitempty"`

	// PostK3sCommands specifies extra commands to run after k3s setup runs
	// +optional
	PostK3sCommands []string `json:"postK3sCommands,omitempty"`

	// AgentConfig specifies configuration for the agent nodes
	// +optional
	AgentConfig KThreesAgentConfig `json:"agentConfig,omitempty"`

	// ServerConfig specifies configuration for the agent nodes
	// +optional
	ServerConfig KThreesServerConfig `json:"serverConfig,omitempty"`

	// Version specifies the k3s version
	// +optional
	Version string `json:"version,omitempty"`
}

// TODO
// Will need extend this func when implementing other k3s database options.
func (c *KThreesConfigSpec) IsEtcdEmbedded() bool {
	return true
}

type KThreesServerConfig struct {
	// KubeAPIServerArgs is a customized flag for kube-apiserver process
	// +optional
	KubeAPIServerArgs []string `json:"kubeAPIServerArg,omitempty"`

	// KubeControllerManagerArgs is a customized flag for kube-controller-manager process
	// +optional
	KubeControllerManagerArgs []string `json:"kubeControllerManagerArgs,omitempty"`

	// KubeSchedulerArgs is a customized flag for kube-scheduler process
	// +optional
	KubeSchedulerArgs []string `json:"kubeSchedulerArgs,omitempty"`

	// TLSSan Add additional hostname or IP as a Subject Alternative Name in the TLS cert
	// +optional
	TLSSan []string `json:"tlsSan,omitempty"`

	// BindAddress k3s bind address (default: 0.0.0.0)
	// +optional
	BindAddress string `json:"bindAddress,omitempty"`

	// HTTPSListenPort HTTPS listen port (default: 6443)
	// +optional
	HTTPSListenPort string `json:"httpsListenPort,omitempty"`

	// AdvertiseAddress IP address that apiserver uses to advertise to members of the cluster (default: node-external-ip/node-ip)
	// +optional
	AdvertiseAddress string `json:"advertiseAddress,omitempty"`

	// AdvertisePort Port that apiserver uses to advertise to members of the cluster (default: listen-port) (default: 0)
	// +optional
	AdvertisePort string `json:"advertisePort,omitempty"`

	// ClusterCidr  Network CIDR to use for pod IPs (default: "10.42.0.0/16")
	// +optional
	ClusterCidr string `json:"clusterCidr,omitempty"`

	// ServiceCidr Network CIDR to use for services IPs (default: "10.43.0.0/16")
	// +optional
	ServiceCidr string `json:"serviceCidr,omitempty"`

	// ClusterDNS  Cluster IP for coredns service. Should be in your service-cidr range (default: 10.43.0.10)
	// +optional
	ClusterDNS string `json:"clusterDNS,omitempty"`

	// ClusterDomain Cluster Domain (default: "cluster.local")
	// +optional
	ClusterDomain string `json:"clusterDomain,omitempty"`

	// DisableComponents  specifies extra commands to run before k3s setup runs
	// +optional
	DisableComponents []string `json:"disableComponents,omitempty"`

	// DisableCloudController disables k3s default cloud controller manager. (default: true)
	// +optional
	DisableCloudController *bool `json:"disableCloudController,omitempty"`

	// CloudProviderName defines the --cloud-provider= kubelet extra arg. (default: "external")
	// +optional
	CloudProviderName *string `json:"cloudProviderName,omitempty"`

	// SystemDefaultRegistry defines private registry to be used for all system images
	// +optional
	SystemDefaultRegistry string `json:"systemDefaultRegistry,omitempty"`

	// Customized etcd proxy image for management cluster to communicate with workload cluster etcd (default: "alpine/socat")
	// +optional
	EtcdProxyImage string `json:"etcdProxyImage,omitempty"`
}

type KThreesAgentConfig struct {
	// NodeLabels  Registering and starting kubelet with set of labels
	// +optional
	NodeLabels []string `json:"nodeLabels,omitempty"`

	// NodeTaints Registering kubelet with set of taints
	// +optional
	NodeTaints []string `json:"nodeTaints,omitempty"`

	// TODO: take in a object or secret and write to file. this is not useful
	// PrivateRegistry  registry configuration file (default: "/etc/rancher/k3s/registries.yaml")
	// +optional
	PrivateRegistry string `json:"privateRegistry,omitempty"`

	// KubeletArgs Customized flag for kubelet process
	// +optional
	KubeletArgs []string `json:"kubeletArgs,omitempty"`

	// KubeProxyArgs Customized flag for kube-proxy process
	// +optional
	KubeProxyArgs []string `json:"kubeProxyArgs,omitempty"`

	// NodeName Name of the Node
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// AirGapped is a boolean value to define if the bootstrapping should be air-gapped,
	// basically supposing that online container registries and k3s install scripts are not reachable.
	// User should prepare docker image, k3s binary, and put the install script in AirGappedInstallScriptPath (default path: "/opt/install.sh")
	// on all nodes in the air-gap environment.
	// +optional
	AirGapped bool `json:"airGapped,omitempty"`

	// AirGappedInstallScriptPath is the path to the install script in the air-gapped environment.
	// The install script should be prepared by the user. The value is only
	// used when AirGapped is set to true (default: "/opt/install.sh").
	// +optional
	AirGappedInstallScriptPath string `json:"airGappedInstallScriptPath,omitempty"`
}

// KThreesConfigStatus defines the observed state of KThreesConfig.
// +kubebuilder:validation:MinProperties=1
type KThreesConfigStatus struct {
	// conditions represents the observations of a KThreesConfig's current state.
	// Known condition types are Ready, DataSecretAvailable, CertificatesAvailable.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// initialization provides observations of the KThreesConfig initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.
	// +optional
	Initialization KThreesConfigInitializationStatus `json:"initialization,omitempty,omitzero"`

	// dataSecretName is the name of the secret that stores the bootstrap data script.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	DataSecretName string `json:"dataSecretName,omitempty"`

	// observedGeneration is the latest generation observed by the controller.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// deprecated groups all the status fields that are deprecated and will be removed when all the nested field are removed.
	// +optional
	Deprecated *KThreesConfigDeprecatedStatus `json:"deprecated,omitempty"`
}

// KThreesConfigInitializationStatus provides observations of the KThreesConfig initialization process.
// +kubebuilder:validation:MinProperties=1
type KThreesConfigInitializationStatus struct {
	// dataSecretCreated is true when the Machine's boostrap secret is created.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Machine provisioning.
	// +optional
	DataSecretCreated *bool `json:"dataSecretCreated,omitempty"`
}

// KThreesConfigDeprecatedStatus groups all the status fields that are deprecated and will be removed in a future version.
type KThreesConfigDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *KThreesConfigV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// KThreesConfigV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
type KThreesConfigV1Beta1DeprecatedStatus struct {
	// conditions defines current service state of the KThreesConfig.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// failureReason will be set on non-retryable errors
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	FailureReason string `json:"failureReason,omitempty"`

	// failureMessage will be set on non-retryable errors
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=10240
	FailureMessage string `json:"failureMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
// +kubebuilder:printcolumn:name="Data secret created",type="boolean",JSONPath=".status.initialization.dataSecretCreated",description="Boostrap secret is created"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of KThreesConfig"

// KThreesConfig is the Schema for the kthreesconfigs API.
type KThreesConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KThreesConfigSpec   `json:"spec,omitempty"`
	Status KThreesConfigStatus `json:"status,omitempty,omitzero"`
}

// GetV1Beta1Conditions returns the set of conditions for this object.
func (c *KThreesConfig) GetV1Beta1Conditions() clusterv1.Conditions {
	if c.Status.Deprecated == nil || c.Status.Deprecated.V1Beta1 == nil {
		return nil
	}
	return c.Status.Deprecated.V1Beta1.Conditions
}

// SetV1Beta1Conditions sets the conditions on this object.
func (c *KThreesConfig) SetV1Beta1Conditions(conditions clusterv1.Conditions) {
	if c.Status.Deprecated == nil {
		c.Status.Deprecated = &KThreesConfigDeprecatedStatus{}
	}
	if c.Status.Deprecated.V1Beta1 == nil {
		c.Status.Deprecated.V1Beta1 = &KThreesConfigV1Beta1DeprecatedStatus{}
	}
	c.Status.Deprecated.V1Beta1.Conditions = conditions
}

// GetConditions returns the set of conditions for this object.
func (c *KThreesConfig) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions sets conditions for an API object.
func (c *KThreesConfig) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// KThreesConfigList contains a list of KThreesConfig.
type KThreesConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KThreesConfig `json:"items"`
}

// Encoding specifies the cloud-init file encoding.
// +kubebuilder:validation:Enum=base64;gzip;gzip+base64
type Encoding string

const (
	// Base64 implies the contents of the file are encoded as base64.
	Base64 Encoding = "base64"
	// Gzip implies the contents of the file are encoded with gzip.
	Gzip Encoding = "gzip"
	// GzipBase64 implies the contents of the file are first base64 encoded and then gzip encoded.
	GzipBase64 Encoding = "gzip+base64"
)

// File defines the input for generating write_files in cloud-init.
type File struct {
	// Path specifies the full path on disk where to store the file.
	Path string `json:"path"`

	// Owner specifies the ownership of the file, e.g. "root:root".
	// +optional
	Owner string `json:"owner,omitempty"`

	// Permissions specifies the permissions to assign to the file, e.g. "0640".
	// +optional
	Permissions string `json:"permissions,omitempty"`

	// Encoding specifies the encoding of the file contents.
	// +optional
	Encoding Encoding `json:"encoding,omitempty"`

	// Content is the actual content of the file.
	// +optional
	Content string `json:"content,omitempty"`

	// ContentFrom is a referenced source of content to populate the file.
	// +optional
	ContentFrom *FileSource `json:"contentFrom,omitempty"`
}

// FileSource is a union of all possible external source types for file data.
// Only one field may be populated in any given instance. Developers adding new
// sources of data for target systems should add them here.
type FileSource struct {
	// Secret represents a secret that should populate this file.
	Secret SecretFileSource `json:"secret"`
}

// Adapts a Secret into a FileSource.
//
// The contents of the target Secret's Data field will be presented
// as files using the keys in the Data field as the file names.
type SecretFileSource struct {
	// Name of the secret in the KThreesBootstrapConfig's namespace to use.
	Name string `json:"name"`

	// Key is the key in the secret's data map for this value.
	Key string `json:"key"`
}

func init() {
	objectTypes = append(objectTypes, &KThreesConfig{}, &KThreesConfigList{})
}
