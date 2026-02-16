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
// XBackendDestination is the Schema for the backends API.
type XBackendDestination struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata"`
	// spec defines the desired state of XBackendDestination.
	// +required
	Spec XBackendDestinationSpec `json:"spec"`
	// status defines the observed state of XBackendDestination.
	// +optional
	Status XBackendDestinationStatus `json:"status"`
}

// +kubebuilder:object:root=true
// XXBackendDestinationList contains a list of XBackendDestination.
type XBackendDestinationList struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is a standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []XBackendDestination `json:"items"`
}

// XBackendDestinationSpec defines the desired state of XBackendDestination.
type XBackendDestinationSpec struct {
	// destination defines the backend destination to route traffic to.
	// +required
	Destination BackendDestination `json:"destination"`
	// extensions defines optional extension processors that can be applied to this backend.
	// +optional
	Extensions []BackendExtension `json:"extensions,omitempty"`
}

// +kubebuilder:validation:ExactlyOneOf=fqdn;service
type BackendDestination struct {
	// +required
	Type BackendType `json:"type"`
	// +required
	// +kubebuilder:validation:MinItems=1
	Ports []BackendPort `json:"ports,omitempty"`
	// +optional
	FQDN *FQDNBackend `json:"fqdn,omitempty"`
	// +optional
	Service *ServiceBackend `json:"service,omitempty"`
}

// BackendType defines the type of the XBackendDestination destination.
// +kubebuilder:validation:Enum=Fqdn;Service
type BackendType string

const (
	// Fqdn represents a fully qualified domain name.
	BackendTypeFqdn BackendType = "Fqdn"
	// Service represents a Kubernetes Service.
	BackendTypeService BackendType = "Service"
)

type BackendPort struct {
	// Number defines the port number of the `XBackendDestination`.
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Number uint32 `json:"number"`
	// Protocol defines the protocol of the `XBackendDestination`.
	// +required
	Protocol BackendProtocol `json:"protocol"`
	// TLS defines the TLS configuration that a client should use when talking to the `XBackendDestination`.
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
	// Mode defines the TLS mode for the XBackendDestination.
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

// ServiceBackend describes a Kubernetes Service backend.
type ServiceBackend struct {
	// Name is the name of the Service.
	// +required
	Name string `json:"name"`
	// Namespace is the namespace of the Service.
	// +required
	Namespace string `json:"namespace"`
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

// XBackendDestinationStatus defines the observed state of XBackendDestination.
type XBackendDestinationStatus struct {
	// Controllers is a list of controllers that are responsible for managing the InferencePoolImport.
	//
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:Required
	Controllers []XBackendDestinationControllerStatus `json:"controllers"`
}

type XBackendDestinationControllerStatus struct {
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
	// conditions represent the current state of the XBackendDestination resource.
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
