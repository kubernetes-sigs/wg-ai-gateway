package envoy

import (
	"context"
	"errors"

	envoyproxytypes "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewaylisters "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1"

	aigatewaylisters "sigs.k8s.io/wg-ai-gateway/k8s/client/listers/api/v0alpha0"
)

type Translator interface {
	TranslateGatewayAndReferencesToXDS(context.Context, *gatewayv1.Gateway) (map[resourcev3.Type][]envoyproxytypes.Resource, error)
}

type translator struct {
	kubeClient    kubernetes.Interface
	gatewayClient gatewayclientset.Interface

	namespaceLister corev1listers.NamespaceLister
	serviceLister   corev1listers.ServiceLister
	secretLister    corev1listers.SecretLister
	gatewayLister   gatewaylisters.GatewayLister
	httprouteLister gatewaylisters.HTTPRouteLister
	backendLister   aigatewaylisters.BackendLister
}

// TODO: Implement translation logic
func New(
	kubeClient kubernetes.Interface,
	gatewayClient gatewayclientset.Interface,
	namespaceLister corev1listers.NamespaceLister,
	serviceLister corev1listers.ServiceLister,
	secretLister corev1listers.SecretLister,
	gatewayLister gatewaylisters.GatewayLister,
	httpRouteLister gatewaylisters.HTTPRouteLister,
	backendLister aigatewaylisters.BackendLister,
) Translator {
	return &translator{
		kubeClient:      kubeClient,
		gatewayClient:   gatewayClient,
		namespaceLister: namespaceLister,
		serviceLister:   serviceLister,
		secretLister:    secretLister,
		gatewayLister:   gatewayLister,
		httprouteLister: httpRouteLister,
		backendLister:   backendLister,
	}
}

// TODO: Take a gateway and its references, and produce the corresponding XDS resources
// Inspired by https://github.com/kubernetes-sigs/kube-agentic-networking/blob/prototype/pkg/translator/translator.go#L95
func (t *translator) TranslateGatewayAndReferencesToXDS(ctx context.Context, gateway *gatewayv1.Gateway) (map[resourcev3.Type][]envoyproxytypes.Resource, error) {
	return nil, errors.New("TODO: implement TranslateGatewayAndReferencesToXDS")
}
