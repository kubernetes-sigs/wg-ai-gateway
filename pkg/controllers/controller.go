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

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	gatewayclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewayinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"
	gatewaylisters "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1"

	aigatewayclientset "sigs.k8s.io/wg-ai-gateway/k8s/client/clientset/versioned"
	aigatewayinformers "sigs.k8s.io/wg-ai-gateway/k8s/client/informers/externalversions"
	aigatewaylisters "sigs.k8s.io/wg-ai-gateway/k8s/client/listers/api/v0alpha0"
)

const (
	numWorkers   = 2
	workInterval = 1 * time.Second
)

type Controller interface {
	Run(context.Context) error
}

type coreResources struct {
	client kubernetes.Interface

	nsLister     corev1listers.NamespaceLister
	svcLister    corev1listers.ServiceLister
	secretLister corev1listers.SecretLister
}

type gatewayResources struct {
	client gatewayclientset.Interface

	gatewayClassLister gatewaylisters.GatewayClassLister
	gatewayLister      gatewaylisters.GatewayLister
	httpRouteLister    gatewaylisters.HTTPRouteLister
}

type aiGatewayResources struct {
	client aigatewayclientset.Interface

	backendLister aigatewaylisters.BackendLister
}

type controller struct {
	core      *coreResources
	gateway   *gatewayResources
	aigateway *aiGatewayResources

	gatewayqueue    workqueue.TypedRateLimitingInterface[string]
	envoyProxyImage string
	syncers         []cache.InformerSynced
	stop            <-chan struct{}
}

func NewController(
	envoyProxyImage string,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	gatewayClient gatewayclientset.Interface,
	aigatewayClient aigatewayclientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	gatewayInformerFactory gatewayinformers.SharedInformerFactory,
	aigatewayInformerFactory aigatewayinformers.SharedInformerFactory,
	stop <-chan struct{},
) (Controller, error) {
	c := &controller{
		core: &coreResources{
			client:       kubeClient,
			nsLister:     kubeInformerFactory.Core().V1().Namespaces().Lister(),
			svcLister:    kubeInformerFactory.Core().V1().Services().Lister(),
			secretLister: kubeInformerFactory.Core().V1().Secrets().Lister(),
		},
		gateway: &gatewayResources{
			client:             gatewayClient,
			gatewayClassLister: gatewayInformerFactory.Gateway().V1().GatewayClasses().Lister(),
			gatewayLister:      gatewayInformerFactory.Gateway().V1().Gateways().Lister(),
			httpRouteLister:    gatewayInformerFactory.Gateway().V1().HTTPRoutes().Lister(),
		},
		aigateway: &aiGatewayResources{
			client:        aigatewayClient,
			backendLister: aigatewayInformerFactory.Ainetworking().V0alpha0().Backends().Lister(),
		},
		stop:            stop,
		envoyProxyImage: envoyProxyImage,
		gatewayqueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "gateway"},
		),
	}

	c.syncers = []cache.InformerSynced{
		kubeInformerFactory.Core().V1().Namespaces().Informer().HasSynced,
		kubeInformerFactory.Core().V1().Services().Informer().HasSynced,
		kubeInformerFactory.Core().V1().Secrets().Informer().HasSynced,
		gatewayInformerFactory.Gateway().V1().GatewayClasses().Informer().HasSynced,
		gatewayInformerFactory.Gateway().V1().Gateways().Informer().HasSynced,
		gatewayInformerFactory.Gateway().V1().HTTPRoutes().Informer().HasSynced,
		aigatewayInformerFactory.Ainetworking().V0alpha0().Backends().Informer().HasSynced,
	}

	// The way we're going to structure this is to have
	if err := c.setupGatewayClassEventHandlers(gatewayInformerFactory.Gateway().V1().GatewayClasses()); err != nil {
		return nil, fmt.Errorf("failed to setup gatewayclass event handlers: %w", err)
	}

	return c, nil
}

func (c *controller) Run(ctx context.Context) error {
	defer runtime.HandleCrashWithContext(ctx)
	defer c.gatewayqueue.ShutDown()

	if ok := cache.WaitForCacheSync(ctx.Done(), c.syncers...); !ok {
		return errors.New("failed to wait for caches to sync")
	}

	for range numWorkers {
		go wait.UntilWithContext(ctx, c.runWorker, workInterval)
	}
	klog.Infof("Started %d workers", numWorkers)

	<-ctx.Done()
	return nil
}

func (c *controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *controller) processNextWorkItem(ctx context.Context) bool {
	item, shouldShutdown := c.gatewayqueue.Get()
	if shouldShutdown {
		return false
	}

	defer c.gatewayqueue.Done(item)

	// We expect strings (namespace/name) to come off the workqueue.
	if err := c.syncHandler(ctx, item); err != nil {
		// Put the item back on the workqueue to handle any transient errors.
		c.gatewayqueue.AddRateLimited(item)
		klog.ErrorS(err, "Error syncing", "key", item)
		return true
	}

	// Finally, if no error occurs we Forget this item so it does not
	// get queued again until another change happens.
	c.gatewayqueue.Forget(item)
	klog.InfoS("Successfully synced", "key", item)
	return true
}

// syncHandler processes a key from the workqueue and reconciles it
func (c *controller) syncHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %w", err))
		return nil
	}

	logger := klog.FromContext(ctx).WithValues("gateway", klog.KRef(namespace, name))
	ctx = klog.NewContext(ctx, logger)

	gateway, err := c.gateway.gatewayLister.Gateways(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Gateway deleted, cleaning up associated resources.")
			return envoy.DeleteProxy(ctx, c.core.client, namespace, name)
		}
		return err
	}

	logger.Info("Syncing gateway")
}
