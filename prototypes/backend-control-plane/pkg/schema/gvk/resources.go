package gvk

// Statically declare all of the resources we know about
var (
	Service                  = GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}
	ConfigMap                = GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}
	Deployment               = GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	DaemonSet                = GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}
	ClusterTrustBundle       = GroupVersionKind{Group: "certificates.k8s.io", Version: "v1beta1", Kind: "ClusterTrustBundle"}
	CustomResourceDefinition = GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}
	EndpointSlice            = GroupVersionKind{Group: "discovery.k8s.io", Version: "v1", Kind: "EndpointSlice"}
	Pod                      = GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	Secret                   = GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}
	ServiceAccount           = GroupVersionKind{Group: "", Version: "v1", Kind: "ServiceAccount"}
	Namespace                = GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}
	Gateway                  = GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "Gateway"}
	GatewayClass             = GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "GatewayClass"}
	HTTPRoute                = GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "HTTPRoute"}
	GRPCRoute                = GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "GRPCRoute"}
	TCPRoute                 = GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1alpha2", Kind: "TCPRoute"}
	TLSRoute                 = GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1alpha2", Kind: "TLSRoute"}
	UDPRoute                 = GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1alpha2", Kind: "UDPRoute"}
	ReferenceGrant           = GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1beta1", Kind: "ReferenceGrant"}
	BackendTLSPolicy         = GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "BackendTLSPolicy"}
	InferencePool            = GroupVersionKind{Group: "inference.networking.k8s.io", Version: "v1", Kind: "InferencePool"}
	ReferenceGrant_v1alpha2  = GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1alpha2", Kind: "ReferenceGrant"}
	XBackendTrafficPolicy    = GroupVersionKind{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Kind: "XBackendTrafficPolicy"}
	XListenerSet             = GroupVersionKind{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Kind: "XListenerSet"}
	Backend                  = GroupVersionKind{Group: "ainetworking.prototype.x-k8s.io", Version: "v0alpha0", Kind: "Backend"}
)
