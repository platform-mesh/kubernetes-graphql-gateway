package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

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
	SecretRef *SercetKeyRef `json:"secretRef,omitempty"`
}

// AuthConfig defines authentication configuration options
// +kubebuilder:validation:XValidation:rule="(has(self.tokenSecretRef) ? 1 : 0) + (has(self.kubeconfigSecretRef) ? 1 : 0) + (has(self.clientCertificateRef) ? 1 : 0) <= 1",message="only one of tokenSecretRef, kubeconfigSecretRef, or clientCertificateRef can be set"
type AuthConfig struct {
	// SecretRef points to a secret containing auth token
	// +optional
	TokenSecretRef *SercetKeyRef `json:"tokenSecretRef,omitempty"`
	// KubeconfigSecretRef points to a secret containing kubeconfig
	// +optional
	KubeconfigSecretRef *SercetKeyRef `json:"kubeconfigSecretRef,omitempty"`
	// ClientCertificateRef points to secrets containing client certificate and key for mTLS
	// Secret must contain tls.crt and tls.key keys.
	// +optional
	ClientCertificateRef *corev1.SecretReference `json:"clientCertificateRef,omitempty"`
}

// SercetKeyRef defines a reference to a secret with a specific key.
type SercetKeyRef struct {
	corev1.SecretReference `json:",inline"`
	// Key is the key in the secret data which contains the token
	// +optional
	Key string `json:"key,omitempty"`
}

// ClusterAccessStatus defines the observed state of ClusterAccess.
type ClusterAccessStatus struct {
	// Conditions represent the latest available observations of the cluster access state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ServiceAccountRef defines a reference to a service account.
type ServiceAccountRef struct {
	Name            string           `json:"name"`
	Namespace       string           `json:"namespace"`
	Audience        []string         `json:"audience,omitempty"`
	TokenExpiration *metav1.Duration `json:"token_expiration,omitempty"`
}

// ClusterAccessList contains a list of ClusterAccess
type ClusterAccessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAccess `json:"items"`
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
