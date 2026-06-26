// Package pkg implements the PayloadProcessor controller.
//
// This file sets up informers, listers, and a reconciliation loop using
// standard client-go patterns (no Istio/KRT dependencies).
package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	discoveryv1listers "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewayinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"
	gatewaylisters "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1"

	"github.com/agentgateway/agentgateway/api"

	ppclient "sigs.k8s.io/wg-ai-gateway/prototypes/payload-processor/api/k8s/client/clientset/versioned"
	ppinformers "sigs.k8s.io/wg-ai-gateway/prototypes/payload-processor/api/k8s/client/informers/externalversions"
	pplisters "sigs.k8s.io/wg-ai-gateway/prototypes/payload-processor/api/k8s/client/listers/api/v0alpha0"

	deployer "sigs.k8s.io/wg-ai-gateway/prototypes/payload-processor/controller/pkg/deployer/agentgateway"
)

// Controller watches PayloadProcessor, Gateway, HTTPRoute, and Service CRDs
// and translates them into agentgateway-compatible xDS resources.
type Controller struct {
	kubeClient kubernetes.Interface
	gwClient   gatewayclient.Interface
	ppClient   ppclient.Interface

	// Informer factories
	kubeInformerFactory informers.SharedInformerFactory
	gwInformerFactory   gatewayinformers.SharedInformerFactory
	ppInformerFactory   ppinformers.SharedInformerFactory

	// Listers
	ppLister            pplisters.PayloadProcessorLister
	gatewayLister       gatewaylisters.GatewayLister
	gatewayClassLister  gatewaylisters.GatewayClassLister
	httpRouteLister     gatewaylisters.HTTPRouteLister
	serviceLister       corev1listers.ServiceLister
	endpointSliceLister discoveryv1listers.EndpointSliceLister

	// Deployer manages agentgateway data plane lifecycle
	deployer *deployer.Deployer

	// Workqueue coalesces rapid-fire informer events into single reconciliations
	queue workqueue.TypedRateLimitingInterface[string]

	// Callback for resource changes
	onResourceChange func()

	// Mutex for thread-safe reconciliation
	mu sync.Mutex
}

// NewController creates a new PayloadProcessor controller with standard client-go informers.
func NewController(
	kubeClient kubernetes.Interface,
	gwClient gatewayclient.Interface,
	ppClient ppclient.Interface,
) *Controller {
	resyncPeriod := 30 * time.Second

	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	gwInformerFactory := gatewayinformers.NewSharedInformerFactory(gwClient, resyncPeriod)
	ppInformerFactory := ppinformers.NewSharedInformerFactory(ppClient, resyncPeriod)

	// TODO(jaellio): Seperate into individual controllers?
	c := &Controller{
		kubeClient:          kubeClient,
		gwClient:            gwClient,
		ppClient:            ppClient,
		kubeInformerFactory: kubeInformerFactory,
		gwInformerFactory:   gwInformerFactory,
		ppInformerFactory:   ppInformerFactory,

		ppLister:            ppInformerFactory.Ainetworking().V0alpha0().PayloadProcessors().Lister(),
		gatewayLister:       gwInformerFactory.Gateway().V1().Gateways().Lister(),
		gatewayClassLister:  gwInformerFactory.Gateway().V1().GatewayClasses().Lister(),
		httpRouteLister:     gwInformerFactory.Gateway().V1().HTTPRoutes().Lister(),
		serviceLister:       kubeInformerFactory.Core().V1().Services().Lister(),
		endpointSliceLister: kubeInformerFactory.Discovery().V1().EndpointSlices().Lister(),

		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "payload-processor"},
		),
	}

	// Register event handlers — any change triggers a full reconciliation
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ interface{}) { c.notify() },
		UpdateFunc: func(_, _ interface{}) { c.notify() },
		DeleteFunc: func(_ interface{}) { c.notify() },
	}

	ppInformerFactory.Ainetworking().V0alpha0().PayloadProcessors().Informer().AddEventHandler(handler)
	gwInformerFactory.Gateway().V1().Gateways().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.notify(); c.handleGatewayEvent(context.Background(), obj, false) },
		UpdateFunc: func(_, obj interface{}) { c.notify(); c.handleGatewayEvent(context.Background(), obj, false) },
		DeleteFunc: func(obj interface{}) { c.notify(); c.handleGatewayEvent(context.Background(), obj, true) },
	})
	gwInformerFactory.Gateway().V1().GatewayClasses().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.handleGatewayClassEvent(context.Background(), obj, false) },
		UpdateFunc: func(_, obj interface{}) { c.handleGatewayClassEvent(context.Background(), obj, false) },
		DeleteFunc: func(obj interface{}) { c.handleGatewayClassEvent(context.Background(), obj, true) },
	})
	gwInformerFactory.Gateway().V1().HTTPRoutes().Informer().AddEventHandler(handler)
	kubeInformerFactory.Core().V1().Services().Informer().AddEventHandler(handler)
	kubeInformerFactory.Discovery().V1().EndpointSlices().Informer().AddEventHandler(handler)

	return c
}

const syncKey = "sync"

func (c *Controller) notify() {
	c.queue.Add(syncKey)
}

// resolveBackend returns a BackendResolver that resolves Service references using listers.
func (c *Controller) resolveBackend() BackendResolver {
	return func(ref gwv1.BackendObjectReference, defaultNamespace string) (*api.BackendReference, error) {
		kind := "Service"
		if ref.Kind != nil {
			kind = string(*ref.Kind)
		}
		if kind != "Service" {
			return nil, fmt.Errorf("unsupported backend kind %q (only Service is supported)", kind)
		}

		namespace := defaultNamespace
		if ref.Namespace != nil {
			namespace = string(*ref.Namespace)
		}
		name := string(ref.Name)

		_, err := c.serviceLister.Services(namespace).Get(name)
		if err != nil {
			return nil, fmt.Errorf("service %s/%s not found: %w", namespace, name, err)
		}

		port := uint32(0)
		if ref.Port != nil {
			port = uint32(*ref.Port)
		}
		hostname := fmt.Sprintf("%s.%s.svc.cluster.local", name, namespace)

		return &api.BackendReference{
			Kind: &api.BackendReference_Service_{
				Service: &api.BackendReference_Service{
					Namespace: namespace,
					Hostname:  hostname,
				},
			},
			Port: port,
		}, nil
	}
}

// AllResources returns all xDS resources (binds, listeners, routes, policies)
// scoped per gateway with the correct naming format.
func (c *Controller) AllResources() []GatewayResource {
	c.mu.Lock()
	defer c.mu.Unlock()

	var all []GatewayResource

	// Gateway resources (bind + listener)
	gateways, _ := c.gatewayLister.List(labels.Everything())
	for _, gw := range gateways {
		all = append(all, TranslateGateway(gw)...)
	}

	// Route resources
	routes, _ := c.httpRouteLister.List(labels.Everything())
	getGateway := func(nn types.NamespacedName) *gwv1.Gateway {
		gw, err := c.gatewayLister.Gateways(nn.Namespace).Get(nn.Name)
		if err != nil {
			return nil
		}
		return gw
	}
	for _, route := range routes {
		all = append(all, TranslateHTTPRoute(route, getGateway)...)
	}

	// Policies from PayloadProcessor CRDs
	pps, _ := c.ppLister.List(labels.Everything())
	resolver := c.resolveBackend()
	for _, pp := range pps {
		policies := TranslatePayloadProcessor(pp, resolver)
		for _, p := range policies {
			if p.Gateway == nil {
				continue
			}
			all = append(all, GatewayResource{
				Gateway:  *p.Gateway,
				Resource: &api.Resource{Kind: &api.Resource_Policy{Policy: p.Policy}},
			})
		}
	}

	return all
}

// AllAddresses returns all Address resources (services + workloads)
// for xDS delivery on the WDS type.
func (c *Controller) AllAddresses() []AddressResource {
	c.mu.Lock()
	defer c.mu.Unlock()

	var all []AddressResource

	// Service addresses
	services, _ := c.serviceLister.List(labels.Everything())
	for _, svc := range services {
		if addr := TranslateServiceToAddress(svc); addr != nil {
			all = append(all, *addr)
		}
	}

	// Workload addresses from endpoint slices
	endpointSlices, _ := c.endpointSliceLister.List(labels.Everything())
	for _, eps := range endpointSlices {
		all = append(all, TranslateEndpointSliceToWorkloads(eps)...)
	}

	return all
}

// SetDeployer configures the gateway data plane deployer.
func (c *Controller) SetDeployer(d *deployer.Deployer) {
	c.deployer = d
}

// Run starts the controller and blocks until the context is cancelled.
func (c *Controller) Run(ctx context.Context) error {
	slog.Info("starting PayloadProcessor controller")
	defer c.queue.ShutDown()

	// Start informer factories
	c.kubeInformerFactory.Start(ctx.Done())
	c.gwInformerFactory.Start(ctx.Done())
	c.ppInformerFactory.Start(ctx.Done())

	// Wait for caches to sync
	slog.Info("waiting for informer caches to sync")
	c.kubeInformerFactory.WaitForCacheSync(ctx.Done())
	c.gwInformerFactory.WaitForCacheSync(ctx.Done())
	c.ppInformerFactory.WaitForCacheSync(ctx.Done())
	slog.Info("informer caches synced")

	// Trigger initial reconciliation
	c.notify()

	// Start worker
	go wait.UntilWithContext(ctx, c.runWorker, 1*time.Second)

	// Block until context is cancelled
	<-ctx.Done()

	slog.Info("PayloadProcessor controller stopped")
	return nil
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(item)

	if c.onResourceChange != nil {
		c.onResourceChange()
	}
	c.queue.Forget(item)
	return true
}

// handleGatewayEvent handles deploying or deleting the agentgateway data plane
// when a Gateway resource changes.
func (c *Controller) handleGatewayEvent(ctx context.Context, obj interface{}, deleted bool) {
	if c.deployer == nil {
		return
	}

	gw, ok := obj.(*gwv1.Gateway)
	if !ok {
		// Handle tombstones
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		gw, ok = tombstone.Obj.(*gwv1.Gateway)
		if !ok {
			return
		}
	}

	if deleted {
		nn := types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name}
		if err := c.deployer.DeleteGateway(ctx, nn); err != nil {
			slog.Error("failed to delete gateway data plane", "gateway", nn, "error", err)
		}
		return
	}

	if string(gw.Spec.GatewayClassName) != deployer.AgentGatewayClassName {
		return
	}

	if err := c.deployer.DeployGateway(ctx, gw); err != nil {
		slog.Error("failed to deploy gateway data plane", "gateway", gw.Name, "error", err)
	}
}

// handleGatewayClassEvent handles GatewayClass reconciliation.
func (c *Controller) handleGatewayClassEvent(ctx context.Context, obj interface{}, deleted bool) {
	gwc, ok := obj.(*gwv1.GatewayClass)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		gwc, ok = tombstone.Obj.(*gwv1.GatewayClass)
		if !ok {
			return
		}
	}
	ReconcileGatewayClassOnChange(ctx, c.gwClient, gwc, deleted)
}

// OnResourceChange registers a callback invoked when any resource changes.
func (c *Controller) OnResourceChange(fn func()) {
	c.onResourceChange = fn
}
