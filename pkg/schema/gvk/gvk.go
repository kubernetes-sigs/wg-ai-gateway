package gvk

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/wg-ai-gateway/pkg/schema/gvr"
)

type GroupVersionKind struct {
	Group   string
	Version string
	Kind    string
}

// ToGVR converts a GroupVersionKind to a GroupVersionResource.
// Returns and empty GVR and false if the GVK is unknown.
func ToGVR(gvk GroupVersionKind) (schema.GroupVersionResource, bool) {
	switch gvk {
	// Core Kubernetes resources
	case Service:
		return gvr.Service, true
	case ConfigMap:
		return gvr.ConfigMap, true
	case Pod:
		return gvr.Pod, true
	case Secret:
		return gvr.Secret, true
	case ServiceAccount:
		return gvr.ServiceAccount, true
	case Namespace:
		return gvr.Namespace, true

	// Apps API resources
	case Deployment:
		return gvr.Deployment, true
	case DaemonSet:
		return gvr.DaemonSet, true

	// Certificate API resources
	case ClusterTrustBundle:
		return gvr.ClusterTrustBundle, true

	// API Extensions resources
	case CustomResourceDefinition:
		return gvr.CustomResourceDefinition, true

	// Discovery API resources
	case EndpointSlice:
		return gvr.EndpointSlice, true

	// Gateway API resources
	case Gateway:
		return gvr.KubernetesGateway, true
	case GatewayClass:
		return gvr.GatewayClass, true
	case HTTPRoute:
		return gvr.HTTPRoute, true
	case GRPCRoute:
		return gvr.GRPCRoute, true
	case TCPRoute:
		return gvr.TCPRoute, true
	case TLSRoute:
		return gvr.TLSRoute, true
	case UDPRoute:
		return gvr.UDPRoute, true
	case ReferenceGrant:
		return gvr.ReferenceGrant, true
	case ReferenceGrant_v1alpha2:
		return gvr.ReferenceGrant_v1alpha2, true
	case BackendTLSPolicy:
		return gvr.BackendTLSPolicy, true

	// Inference API resources
	case InferencePool:
		return gvr.InferencePool, true

	// Extended Gateway API resources
	case XBackendTrafficPolicy:
		return gvr.XBackendTrafficPolicy, true
	case XListenerSet:
		return gvr.XListenerSet, true

	// AI Networking prototype resources
	case Backend:
		return gvr.Backend, true

	default:
		return schema.GroupVersionResource{}, false
	}
}
