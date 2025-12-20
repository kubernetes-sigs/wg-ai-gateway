/*
Copyright 2025 The Kubernetes Authors.

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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gateway "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// Backend is the Schema for the backends API.
type Backend struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata"`
	// spec defines the desired state of Backend.
	// +required
	Spec BackendSpec `json:"spec"`
	// status defines the observed state of Backend.
	// +optional
	Status BackendStatus `json:"status"`
}

// +kubebuilder:object:root=true
// XBackendList contains a list of Backend.
type BackendList struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is a standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backend `json:"items"`
}

// BackendSpec defines the desired state of Backend.
type BackendSpec struct {
	// destination defines the backend destination to route traffic to.
	// +required
	Destination BackendDestination `json:"destination"`
	// extensions defines optional extension processors that can be applied to this backend.
	// +optional
	Extensions []BackendExtension `json:"extensions,omitempty"`
}

// TODO: Do we need the destination field or can we inline this all
// in spec?
// +kubebuilder:validation:ExactlyOneOf=fqdn;service;ip
type BackendDestination struct {
	// +required
	Type BackendType `json:"type"`
	// +optional
	Ports []BackendPort `json:"ports,omitempty"`
	// +optional
	FQDN *FQDNBackend `json:"fqdn,omitempty"`
	// Service *ServiceBackend `json:"service,omitempty"`
	// IP *IPBackend `json:"ip,omitempty"`
}

// BackendType defines the type of the Backend destination.
// +kubebuilder:validation:Enum=Fqdn;Ip;KubernetesService
type BackendType string

const (
	// Fqdn represents a fully qualified domain name.
	BackendTypeFqdn BackendType = "Fqdn"
	// Ip represents an IP address.
	BackendTypeIp BackendType = "Ip"
	// KubernetesService represents a Kubernetes Service.
	BackendTypeKubernetesService BackendType = "KubernetesService"
)

type BackendPort struct {
	// Number defines the port number of the backend.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Number uint32 `json:"number"`
	// Protocol defines the protocol of the backend.
	// +required
	Protocol BackendProtocol `json:"protocol"`
	// TLS defines the TLS configuration that a client should use when talking to the backend.
	// TODO: To prevent duplication on the part of the user, maybe this should be declared once at the
	// top level with per-port overrides?
	// +optional
	TLS *BackendTLS `json:"tls,omitempty"`
	// +optional
	ProtocolOptions *BackendProtocolOptions `json:"protocolOptions,omitempty"`
}

// BackendProtocol defines the protocol for backend communication.
// +kubebuilder:validation:Enum=HTTP;HTTP2;TCP;MCP
// +kubebuilder:validation:MaxLength=256
type BackendProtocol string

const (
	BackendProtocolHTTP  BackendProtocol = "HTTP"
	BackendProtocolHTTP2 BackendProtocol = "HTTP2"
	BackendProtocolTCP   BackendProtocol = "TCP"
	BackendProtocolMCP   BackendProtocol = "MCP"
)

type BackendTLS struct {
	// Mode defines the TLS mode for the backend.
	// +required
	Mode BackendTLSMode `json:"mode"`
	// SNI defines the server name indication to present to the upstream backend.
	// +optional
	SNI string `json:"sni,omitempty"`
	// CaBundleRef defines the reference to the CA bundle for validating the backend's
	// certificate.
	// Defaults to system CAs if not specified.
	// +optional
	CaBundleRef []gateway.ObjectReference `json:"caBundleRef,omitempty"`

	InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty"`

	// ClientCertificateRef defines the reference to the client certificate for mutual
	// TLS. Only used if mode is MUTUAL.
	// +optional
	ClientCertificateRef *gateway.SecretObjectReference `json:"clientCertificateRef,omitempty"`

	SubjectAltNames []string `json:"subjectAltNames,omitempty"`
}

// BackendTLSMode defines the TLS mode for backend connections.
// +kubebuilder:validation:Enum=Simple;Mutual;None
type BackendTLSMode string

const (
	// Do not modify or configure TLS. If your platform (or service mesh)
	// transparently handles TLS, use this mode.
	BackendTLSModeNone BackendTLSMode = "None"
	// Enable TLS with simple server certificate verification.
	BackendTLSModeSimple BackendTLSMode = "Simple"
	// Enable mutual TLS.
	BackendTLSModeMutual BackendTLSMode = "Mutual"
)

// +kubebuilder:validation:ExactlyOneOf=mcp
type BackendProtocolOptions struct {
	// +optional
	MCP *MCPProtocolOptions `json:"mcp,omitempty"`
}

type MCPProtocolOptions struct {
	// MCP protocol version. MUST be a valid MCP version string
	// per the project's strategy: https://modelcontextprotocol.io/specification/versioning
	// +optional
	// +kubebuilder:validation:MaxLength=256
	Version string `json:"version,omitempty"`
	// URL path for MCP traffic. Default is /mcp.
	// +optional
	// +kubebuilder:default:=/mcp
	Path string `json:"path,omitempty"`
}

// FQDNBackend describes a backend that exists outside of the cluster.
// Hostnames must not be cluster.local domains or otherwise refer to
// Kubernetes services within a cluster. Implementations must report
// violations of this requirement in status.
type FQDNBackend struct {
	// Hostname of the backend service. Examples: "api.example.com"
	// +required
	Hostname string `json:"hostname"`
}

type BackendExtension struct {
	// +required
	Name string `json:"name"`
	// +required
	Type string `json:"type"`
	// +optional
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	RawConfig *apiextensionsv1.JSON `json:"rawConfig,omitempty"`
}

// BackendStatus defines the observed state of Backend.
type BackendStatus struct {
	// Controllers is a list of controllers that are responsible for managing the InferencePoolImport.
	//
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:Required
	Controllers []BackendControllerStatus `json:"controllers"`
}

type BackendControllerStatus struct {
	// Name is a domain/path string that indicates the name of the controller that manages the
	// InferencePoolImport. Name corresponds to the GatewayClass controllerName field when the
	// controller will manage parents of type "Gateway". Otherwise, the name is implementation-specific.
	//
	// Example: "example.net/import-controller".
	//
	// The format of this field is DOMAIN "/" PATH, where DOMAIN and PATH are valid Kubernetes
	// names (https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names).
	//
	// A controller MUST populate this field when writing status and ensure that entries to status
	// populated with their controller name are removed when they are no longer necessary.
	//
	// +required
	Name gateway.GatewayController `json:"name"`
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
	// conditions represent the current state of the Backend resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
