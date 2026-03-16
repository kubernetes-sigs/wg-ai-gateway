/*
Copyright 2026 The Kubernetes Authors.

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

// IMPORTANT: Run "make generate" to regenerate code after modifying this file

package v0alpha0

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// EgressGateway is a dedicated gateway resource for egress traffic.
// Unlike the standard Gateway resource, EgressGateway omits the ingress-oriented
// Hostname field on listeners, providing clearer semantics and RBAC boundaries
// for egress use cases. Addresses are retained so that an EgressGateway can
// represent an external proxy reachable at a known IP or hostname.
type EgressGateway struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata"`
	// spec defines the desired state of EgressGateway.
	// +required
	Spec EgressGatewaySpec `json:"spec"`
	// status defines the observed state of EgressGateway.
	// +optional
	Status EgressGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// EgressGatewayList contains a list of EgressGateway.
type EgressGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is a standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EgressGateway `json:"items"`
}

// EgressGatewaySpec defines the desired state of EgressGateway.
// This is a subset of the Gateway API GatewaySpec, omitting the ingress-oriented
// Hostname field on listeners.
type EgressGatewaySpec struct {
	// gatewayClassName is the name of the GatewayClass used by this EgressGateway.
	// +required
	GatewayClassName gatewayv1.ObjectName `json:"gatewayClassName"`

	// addresses defines the network addresses that this EgressGateway is
	// associated with. For egress, this represents the address of an external
	// proxy (e.g. an out-of-cluster egress proxy) so that workloads can
	// discover where to route traffic via the EgressGateway API.
	// +optional
	// +kubebuilder:validation:MaxItems=16
	Addresses []gatewayv1.GatewaySpecAddress `json:"addresses,omitempty"`

	// listeners defines the set of listeners for this EgressGateway.
	// Unlike Gateway listeners, EgressGateway listeners do not have a Hostname field
	// since egress traffic does not have a frontend hostname.
	// +required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	Listeners []EgressListener `json:"listeners"`

	// infrastructure defines infrastructure-level attributes about this EgressGateway instance.
	// +optional
	Infrastructure *gatewayv1.GatewayInfrastructure `json:"infrastructure,omitempty"`

	// backendTLS defines gateway-wide default TLS settings for backend connections.
	// Individual backends can override these defaults.
	// +optional
	BackendTLS *EgressGatewayBackendTLS `json:"backendTLS,omitempty"`
}

// EgressListener defines a listener for the EgressGateway.
// Unlike a standard Gateway listener, it does not include a Hostname field
// because egress traffic does not have a frontend hostname.
type EgressListener struct {
	// name is the name of the listener.
	// +required
	Name gatewayv1.SectionName `json:"name"`

	// port is the port number on which the listener listens.
	// +required
	Port gatewayv1.PortNumber `json:"port"`

	// protocol is the protocol that the listener accepts.
	// +required
	Protocol gatewayv1.ProtocolType `json:"protocol"`

	// tls defines TLS configuration for this listener (frontend termination).
	// +optional
	TLS *gatewayv1.ListenerTLSConfig `json:"tls,omitempty"`

	// allowedRoutes defines which routes can attach to this listener.
	// +optional
	AllowedRoutes *gatewayv1.AllowedRoutes `json:"allowedRoutes,omitempty"`
}

// EgressGatewayBackendTLS defines gateway-wide default TLS settings for backend connections.
type EgressGatewayBackendTLS struct {
	// mode defines the TLS mode for backend connections.
	// +required
	Mode BackendTLSMode `json:"mode"`

	// caBundleRef defines references to CA bundles for validating backend certificates.
	// Defaults to system CAs if not specified.
	// +optional
	CaBundleRef []gatewayv1.ObjectReference `json:"caBundleRef,omitempty"`

	// insecureSkipVerify disables TLS certificate verification for backend connections.
	// +optional
	InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty"`

	// clientCertificateRef defines the reference to the client certificate for mutual TLS.
	// +optional
	ClientCertificateRef *gatewayv1.SecretObjectReference `json:"clientCertificateRef,omitempty"`

	// subjectAltNames defines the acceptable subject alternative names for backend certificates.
	// +optional
	SubjectAltNames []string `json:"subjectAltNames,omitempty"`
}

// EgressGatewayStatus defines the observed state of EgressGateway.
type EgressGatewayStatus struct {
	// addresses lists the network addresses that have been bound to or
	// associated with this EgressGateway.
	// +optional
	// +kubebuilder:validation:MaxItems=16
	Addresses []gatewayv1.GatewayStatusAddress `json:"addresses,omitempty"`

	// conditions describe the current conditions of the EgressGateway.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=8
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// listeners provides status for each listener defined in the spec.
	// +optional
	Listeners []gatewayv1.ListenerStatus `json:"listeners,omitempty"`
}
