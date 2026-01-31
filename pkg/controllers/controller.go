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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewayinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"
	gatewaylisters "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1"

	aigatewayclientset "sigs.k8s.io/wg-ai-gateway/k8s/client/clientset/versioned"
	aigatewayinformers "sigs.k8s.io/wg-ai-gateway/k8s/client/informers/externalversions"
	aigatewaylisters "sigs.k8s.io/wg-ai-gateway/k8s/client/listers/api/v0alpha0"
	envoydeployer "sigs.k8s.io/wg-ai-gateway/pkg/deployer/envoy"
	envoytranslator "sigs.k8s.io/wg-ai-gateway/pkg/translator/envoy"
	envoycontrolplane "sigs.k8s.io/wg-ai-gateway/pkg/xds/envoy"
)

const (
	numWorkers   = 2
	workInterval = 1 * time.Second
)

type Controller interface {
	Run(context.Context) error
}

type coreResources struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface

	nsLister             corev1listers.NamespaceLister
	svcLister            corev1listers.ServiceLister
	secretLister         corev1listers.SecretLister
	configMapLister      corev1listers.ConfigMapLister
	serviceAccountLister corev1listers.ServiceAccountLister
	deploymentLister     appv1listers.DeploymentLister
	serviceLister        corev1listers.ServiceLister
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
	controlplane    envoycontrolplane.ControlPlane
	translator      envoytranslator.Translator
	stop            <-chan struct{}
}

func NewController(
	ctx context.Context,
	envoyProxyImage string,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	gatewayClient gatewayclientset.Interface,
	aigatewayClient aigatewayclientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	gatewayInformerFactory gatewayinformers.SharedInformerFactory,
	aigatewayInformerFactory aigatewayinformers.SharedInformerFactory,
) (Controller, error) {
	c := &controller{
		core: &coreResources{
			client:               kubeClient,
			dynamicClient:        dynamicClient,
			nsLister:             kubeInformerFactory.Core().V1().Namespaces().Lister(),
			svcLister:            kubeInformerFactory.Core().V1().Services().Lister(),
			secretLister:         kubeInformerFactory.Core().V1().Secrets().Lister(),
			configMapLister:      kubeInformerFactory.Core().V1().ConfigMaps().Lister(),
			serviceAccountLister: kubeInformerFactory.Core().V1().ServiceAccounts().Lister(),
			deploymentLister:     kubeInformerFactory.Apps().V1().Deployments().Lister(),
			serviceLister:        kubeInformerFactory.Core().V1().Services().Lister(),
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
		stop:            ctx.Done(),
		envoyProxyImage: envoyProxyImage,
		gatewayqueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "gateway"},
		),
		controlplane: envoycontrolplane.NewControlPlane(ctx),
		translator: envoytranslator.New(
			kubeClient,
			gatewayClient,
			kubeInformerFactory.Core().V1().Namespaces().Lister(),
			kubeInformerFactory.Core().V1().Services().Lister(),
			kubeInformerFactory.Core().V1().Secrets().Lister(),
			kubeInformerFactory.Discovery().V1().EndpointSlices().Lister(),
			gatewayInformerFactory.Gateway().V1().Gateways().Lister(),
			gatewayInformerFactory.Gateway().V1().HTTPRoutes().Lister(),
			aigatewayInformerFactory.Ainetworking().V0alpha0().Backends().Lister(),
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

	// Set up event handlers for Gateway API resources
	if err := c.setupGatewayClassEventHandlers(gatewayInformerFactory.Gateway().V1().GatewayClasses()); err != nil {
		return nil, fmt.Errorf("failed to setup gatewayclass event handlers: %w", err)
	}

	if err := c.setupGatewayEventHandlers(gatewayInformerFactory.Gateway().V1().Gateways()); err != nil {
		return nil, fmt.Errorf("failed to setup gateway event handlers: %w", err)
	}

	if err := c.setupHTTPRouteEventHandlers(gatewayInformerFactory.Gateway().V1().HTTPRoutes()); err != nil {
		return nil, fmt.Errorf("failed to setup httproute event handlers: %w", err)
	}

	return c, nil
}

func (c *controller) Run(ctx context.Context) error {
	defer runtime.HandleCrashWithContext(ctx)
	defer c.gatewayqueue.ShutDown()

	// Note: control plane Run() is non-blocking so it's
	// safe to run in this goroutine
	if err := c.controlplane.Run(ctx); err != nil {
		return fmt.Errorf("failed to start xDS server: %w", err)
	}

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
			return envoydeployer.DeleteGatewayInfra(ctx, c.core.client, types.NamespacedName{Namespace: namespace, Name: name})
		}
		return err
	}

	logger.Info("Syncing gateway")

	deployer := envoydeployer.NewDeployer(
		c.core.client,
		c.core.dynamicClient,
		gateway,
		c.envoyProxyImage,
		c.core.configMapLister,
		c.core.serviceAccountLister,
		c.core.serviceLister,
		c.core.deploymentLister,
	)
	if err := deployer.Deploy(ctx); err != nil {
		return fmt.Errorf("failed to deploy gateway infrastructure: %w", err)
	}

	logger.Info("Reconciled gateway successfully")

	// Translate Gateway to xDS resources.
	resources, httpRouteStatuses, err := c.translator.TranslateGatewayAndReferencesToXDS(ctx, gateway)
	if err != nil {
		if statusErr := c.updateGatewayStatus(ctx, gateway, metav1.ConditionFalse, "TranslationError", err.Error()); statusErr != nil {
			logger.Error(statusErr, "failed to update gateway status with translation error")
		}
		return fmt.Errorf("failed to translate gateway to xDS resources: %w", err)
	}

	logger.Info("Translated gateway to xDS resources")

	// Update the xDS server with the new resources.
	if err := c.controlplane.PushXDS(ctx, deployer.NodeID(), resources); err != nil {
		return fmt.Errorf("failed to update xDS server: %w", err)
	}

	logger.Info("Updated xDS server with new resources", "nodeID", deployer.NodeID())

	// Update Gateway status to indicate successful programming
	if err := c.updateGatewayStatus(ctx, gateway, metav1.ConditionTrue, "Programmed", "Gateway is programmed and ready"); err != nil {
		logger.Error(err, "failed to update gateway status")
		// Don't return error as the gateway is actually working
	}

	// Update HTTPRoute statuses
	for httpRouteKey, parentStatuses := range httpRouteStatuses {
		if err := c.updateHTTPRouteStatus(ctx, httpRouteKey, parentStatuses); err != nil {
			logger.Error(err, "failed to update httproute status", "httproute", httpRouteKey)
			// Don't return error as the routes are actually working
		}
	}

	return nil
}

// updateGatewayStatus updates the Gateway status with the given condition.
func (c *controller) updateGatewayStatus(ctx context.Context, gateway *gatewayv1.Gateway, status metav1.ConditionStatus, reason, message string) error {
	// Create a copy to avoid modifying the cached object
	gatewayCopy := gateway.DeepCopy()

	// Set the Accepted condition
	apimeta.SetStatusCondition(&gatewayCopy.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             "Accepted",
		Message:            "Gateway configuration is valid",
		LastTransitionTime: metav1.Now(),
	})

	// Set the Programmed condition
	apimeta.SetStatusCondition(&gatewayCopy.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionProgrammed),
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})

	// Set the address if the gateway is programmed successfully
	if status == metav1.ConditionTrue {
		// Find the LoadBalancer service for this gateway and get its external IP
		deployer := envoydeployer.NewDeployer(
			c.core.client,
			c.core.dynamicClient,
			gateway,
			"", // image not needed for getting the service
			c.core.configMapLister,
			c.core.serviceAccountLister,
			c.core.serviceLister,
			c.core.deploymentLister,
		)

		// Try to get the service and extract the LoadBalancer IP
		serviceName := deployer.NodeID()
		service, err := c.core.serviceLister.Services(gateway.Namespace).Get(serviceName)
		if err == nil && service.Spec.Type == "LoadBalancer" && len(service.Status.LoadBalancer.Ingress) > 0 {
			// Set addresses from LoadBalancer ingress
			gatewayCopy.Status.Addresses = []gatewayv1.GatewayStatusAddress{}
			for _, ingress := range service.Status.LoadBalancer.Ingress {
				if ingress.IP != "" {
					gatewayCopy.Status.Addresses = append(gatewayCopy.Status.Addresses, gatewayv1.GatewayStatusAddress{
						Type:  &[]gatewayv1.AddressType{gatewayv1.IPAddressType}[0],
						Value: ingress.IP,
					})
				}
			}
		}
	}

	// Update the Gateway status
	_, err := c.gateway.client.GatewayV1().Gateways(gateway.Namespace).UpdateStatus(ctx, gatewayCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update gateway status: %w", err)
	}

	return nil
}

// updateHTTPRouteStatus updates the HTTPRoute status with the given parent statuses.
func (c *controller) updateHTTPRouteStatus(ctx context.Context, httpRouteKey types.NamespacedName, parentStatuses []gatewayv1.RouteParentStatus) error {
	// Get the current HTTPRoute
	httpRoute, err := c.gateway.httpRouteLister.HTTPRoutes(httpRouteKey.Namespace).Get(httpRouteKey.Name)
	if err != nil {
		return fmt.Errorf("failed to get httproute: %w", err)
	}

	// Create a copy to avoid modifying the cached object
	httpRouteCopy := httpRoute.DeepCopy()

	// Set the parent statuses
	httpRouteCopy.Status.Parents = parentStatuses

	// Update the HTTPRoute status
	_, err = c.gateway.client.GatewayV1().HTTPRoutes(httpRoute.Namespace).UpdateStatus(ctx, httpRouteCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update httproute status: %w", err)
	}

	return nil
}
