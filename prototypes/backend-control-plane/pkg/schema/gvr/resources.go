package gvr

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	BackendTLSPolicy         = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "backendtlspolicies"}
	ClusterTrustBundle       = schema.GroupVersionResource{Group: "certificates.k8s.io", Version: "v1beta1", Resource: "clustertrustbundles"}
	ConfigMap                = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	CustomResourceDefinition = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	DaemonSet                = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
	Deployment               = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	EndpointSlice            = schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}
	GRPCRoute                = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "grpcroutes"}
	GatewayClass             = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gatewayclasses"}
	HTTPRoute                = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	InferencePool            = schema.GroupVersionResource{Group: "inference.networking.k8s.io", Version: "v1", Resource: "inferencepools"}
	KubernetesGateway        = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	Namespace                = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	Pod                      = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	ReferenceGrant           = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "referencegrants"}
	ReferenceGrant_v1alpha2  = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1alpha2", Resource: "referencegrants"}
	Secret                   = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	Service                  = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
	ServiceAccount           = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"}
	TCPRoute                 = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1alpha2", Resource: "tcproutes"}
	TLSRoute                 = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1alpha2", Resource: "tlsroutes"}
	UDPRoute                 = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1alpha2", Resource: "udproutes"}
	XBackendTrafficPolicy    = schema.GroupVersionResource{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Resource: "xbackendtrafficpolicies"}
	XListenerSet             = schema.GroupVersionResource{Group: "gateway.networking.x-k8s.io", Version: "v1alpha1", Resource: "xlistenersets"}
	Backend                  = schema.GroupVersionResource{Group: "ainetworking.prototype.x-k8s.io", Version: "v0alpha0", Resource: "backends"}
)
