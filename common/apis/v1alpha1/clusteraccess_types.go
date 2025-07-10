package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterAccessSpec defines the desired state of ClusterAccess
type ClusterAccessSpec struct {
	// Path is an optional field. If not set, the name of the resource is used
	// +optional
	Path string `json:"path,omitempty"`

	// Host is the URL for the cluster
	Host string `json:"host"`

	// CA configuration for the cluster
	// +optional
	CA *CAConfig `json:"ca,omitempty"`

	// Auth configuration for the cluster
	// +optional
	Auth *AuthConfig `json:"auth,omitempty"`
}

// CAConfig defines CA configuration options
type CAConfig struct {
	// SecretRef points to a secret containing CA data
	// +optional
	SecretRef *SecretRef `json:"secretRef,omitempty"`

	// ConfigMapRef points to a config map containing CA data
	// +optional
	ConfigMapRef *ConfigMapRef `json:"configMapRef,omitempty"`
}

// AuthConfig defines authentication configuration options
type AuthConfig struct {
	// SecretRef points to a secret containing auth token
	// +optional
	SecretRef *SecretRef `json:"secretRef,omitempty"`

	// KubeconfigSecretRef points to a secret containing kubeconfig
	// +optional
	KubeconfigSecretRef *KubeconfigSecretRef `json:"kubeconfigSecretRef,omitempty"`

	// ServiceAccount is the name of the service account to use
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// ClientCertificateRef points to secrets containing client certificate and key for mTLS
	// +optional
	ClientCertificateRef *ClientCertificateRef `json:"clientCertificateRef,omitempty"`
}

// SecretRef defines a reference to a secret
type SecretRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Key       string `json:"key"`
}

// ConfigMapRef defines a reference to a config map
type ConfigMapRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Key       string `json:"key"`
}

// KubeconfigSecretRef defines a reference to a kubeconfig secret
type KubeconfigSecretRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// ClientCertificateRef defines a reference to a client certificate secret
type ClientCertificateRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// ClusterAccessStatus defines the observed state of ClusterAccess
type ClusterAccessStatus struct {
	// Conditions represent the latest available observations of the cluster access state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster,shortName=ca

// ClusterAccess is the Schema for the clusteraccesses API
type ClusterAccess struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterAccessSpec   `json:"spec,omitempty"`
	Status ClusterAccessStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterAccessList contains a list of ClusterAccess
type ClusterAccessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAccess `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterAccess{}, &ClusterAccessList{})
}

// GetConditions returns the conditions from the ClusterAccess status
// This method implements the RuntimeObjectConditions interface
func (ca *ClusterAccess) GetConditions() []metav1.Condition {
	return ca.Status.Conditions
}

// SetConditions sets the conditions in the ClusterAccess status
// This method implements the RuntimeObjectConditions interface
func (ca *ClusterAccess) SetConditions(conditions []metav1.Condition) {
	ca.Status.Conditions = conditions
}
