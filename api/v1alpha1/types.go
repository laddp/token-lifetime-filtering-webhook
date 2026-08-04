package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// SingletonName is the only TokenLifetimePolicy name honored by the webhook.
	SingletonName = "cluster"

	// DefaultMaxExpiration is used when spec.maxExpiration is empty.
	DefaultMaxExpiration = "1y"

	// OnExceedDeny rejects TokenRequests that exceed maxExpiration.
	OnExceedDeny OnExceed = "Deny"
	// OnExceedMutate patches expirationSeconds down to maxExpiration.
	OnExceedMutate OnExceed = "Mutate"
)

// OnExceed controls behavior when a TokenRequest exceeds maxExpiration.
// +kubebuilder:validation:Enum=Deny;Mutate
type OnExceed string

// TokenLifetimePolicySpec defines the desired state of TokenLifetimePolicy.
type TokenLifetimePolicySpec struct {
	// MaxExpiration is the maximum allowed TokenRequest lifetime.
	// Human-readable durations including composites are accepted (s, m, h, d, w, y),
	// e.g. "24h", "7d", "1h30m", "1y".
	// Defaults to "1y" when empty.
	// +optional
	// +kubebuilder:default="1y"
	MaxExpiration string `json:"maxExpiration,omitempty"`

	// OnExceed controls whether over-limit requests are denied or mutated to the max.
	// Defaults to Deny.
	// +optional
	// +kubebuilder:default=Deny
	OnExceed OnExceed `json:"onExceed,omitempty"`

	// ExemptNamespaces is a list of shell-style globs. Matching namespaces skip enforcement.
	// Empty means no exemptions (enforce in all namespaces).
	// +optional
	ExemptNamespaces []string `json:"exemptNamespaces,omitempty"`
}

// TokenLifetimePolicyStatus defines the observed state of TokenLifetimePolicy.
type TokenLifetimePolicyStatus struct{}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=tlp
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="TokenLifetimePolicy is a singleton; metadata.name must be 'cluster'"

// TokenLifetimePolicy is the Schema for the tokenlifetimepolicies API.
type TokenLifetimePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TokenLifetimePolicySpec   `json:"spec,omitempty"`
	Status TokenLifetimePolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TokenLifetimePolicyList contains a list of TokenLifetimePolicy.
type TokenLifetimePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TokenLifetimePolicy `json:"items"`
}
