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

	v0alpha0 "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/api/v0alpha0"
	aigatewayclientset "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/k8s/client/clientset/versioned"
	aigatewayinformers "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/k8s/client/informers/externalversions"
	aigatewaylisters "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/k8s/client/listers/api/v0alpha0"
	envoydeployer "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/pkg/deployer/envoy"
	envoytranslator "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/pkg/translator/envoy"
	envoycontrolplane "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/pkg/xds/envoy"
)

const (
	numWorkers   = 2
	workInterval = 1 * time.Second

	// EgressGatewayClassName is the GatewayClass name that this controller manages.
	EgressGatewayClassName = "wg-ai-egress-gateway"

	// EgressEnvoyControllerName is the controller name for the egress gateway.
	EgressEnvoyControllerName = "sigs.k8s.io/wg-ai-egress-gateway-envoy-controller"
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

	httpRouteLister gatewaylisters.HTTPRouteLister
}

type aiGatewayResources struct {
	client aigatewayclientset.Interface

	egressGatewayLister aigatewaylisters.EgressGatewayLister
	backendLister       aigatewaylisters.XBackendDestinationLister
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
			client:          gatewayClient,
			httpRouteLister: gatewayInformerFactory.Gateway().V1().HTTPRoutes().Lister(),
		},
		aigateway: &aiGatewayResources{
			client:              aigatewayClient,
			egressGatewayLister: aigatewayInformerFactory.Ainetworking().V0alpha0().EgressGateways().Lister(),
			backendLister:       aigatewayInformerFactory.Ainetworking().V0alpha0().XBackendDestinations().Lister(),
		},
		stop:            ctx.Done(),
		envoyProxyImage: envoyProxyImage,
		gatewayqueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "egressgateway"},
		),
		controlplane: envoycontrolplane.NewControlPlane(ctx),
		translator: envoytranslator.New(
			kubeClient,
			gatewayClient,
			kubeInformerFactory.Core().V1().Namespaces().Lister(),
			kubeInformerFactory.Core().V1().Services().Lister(),
			kubeInformerFactory.Core().V1().Secrets().Lister(),
			kubeInformerFactory.Discovery().V1().EndpointSlices().Lister(),
			// The translator uses a gateway lister for internal lookups; we pass
			// the standard Gateway informer even though we don't watch Gateways
			// ourselves (the translator may reference it internally).
			gatewayInformerFactory.Gateway().V1().Gateways().Lister(),
			gatewayInformerFactory.Gateway().V1().HTTPRoutes().Lister(),
			aigatewayInformerFactory.Ainetworking().V0alpha0().XBackendDestinations().Lister(),
		),
	}

	c.syncers = []cache.InformerSynced{
		kubeInformerFactory.Core().V1().Namespaces().Informer().HasSynced,
		kubeInformerFactory.Core().V1().Services().Informer().HasSynced,
		kubeInformerFactory.Core().V1().Secrets().Informer().HasSynced,
		gatewayInformerFactory.Gateway().V1().HTTPRoutes().Informer().HasSynced,
		aigatewayInformerFactory.Ainetworking().V0alpha0().EgressGateways().Informer().HasSynced,
		aigatewayInformerFactory.Ainetworking().V0alpha0().XBackendDestinations().Informer().HasSynced,
	}

	if err := c.setupEgressGatewayEventHandlers(aigatewayInformerFactory.Ainetworking().V0alpha0().EgressGateways()); err != nil {
		return nil, fmt.Errorf("failed to setup egressgateway event handlers: %w", err)
	}

	if err := c.setupHTTPRouteEventHandlers(gatewayInformerFactory.Gateway().V1().HTTPRoutes()); err != nil {
		return nil, fmt.Errorf("failed to setup httproute event handlers: %w", err)
	}

	return c, nil
}

func (c *controller) Run(ctx context.Context) error {
	defer runtime.HandleCrashWithContext(ctx)
	defer c.gatewayqueue.ShutDown()

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

	if err := c.syncHandler(ctx, item); err != nil {
		c.gatewayqueue.AddRateLimited(item)
		klog.ErrorS(err, "Error syncing", "key", item)
		return true
	}

	c.gatewayqueue.Forget(item)
	klog.InfoS("Successfully synced", "key", item)
	return true
}

// syncHandler processes a key from the workqueue and reconciles it.
func (c *controller) syncHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %w", err))
		return nil
	}

	logger := klog.FromContext(ctx).WithValues("egressgateway", klog.KRef(namespace, name))
	ctx = klog.NewContext(ctx, logger)

	egressGateway, err := c.aigateway.egressGatewayLister.EgressGateways(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("EgressGateway deleted, cleaning up associated resources.")
			return envoydeployer.DeleteGatewayInfra(ctx, c.core.client, types.NamespacedName{Namespace: namespace, Name: name})
		}
		return err
	}

	logger.Info("Syncing EgressGateway")

	// Convert EgressGateway to a standard Gateway for reuse with existing translator/deployer
	gateway := egressGatewayToGateway(egressGateway)

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
		return fmt.Errorf("failed to deploy EgressGateway infrastructure: %w", err)
	}

	logger.Info("Reconciled EgressGateway successfully")

	// Translate the converted Gateway to xDS resources
	resources, httpRouteStatuses, err := c.translator.TranslateGatewayAndReferencesToXDS(ctx, gateway)
	if err != nil {
		if statusErr := c.updateEgressGatewayStatus(ctx, egressGateway, metav1.ConditionFalse, "TranslationError", err.Error()); statusErr != nil {
			logger.Error(statusErr, "failed to update EgressGateway status with translation error")
		}
		return fmt.Errorf("failed to translate EgressGateway to xDS resources: %w", err)
	}

	logger.Info("Translated EgressGateway to xDS resources")

	if err := c.controlplane.PushXDS(ctx, deployer.NodeID(), resources); err != nil {
		return fmt.Errorf("failed to update xDS server: %w", err)
	}

	logger.Info("Updated xDS server with new resources", "nodeID", deployer.NodeID())

	if err := c.updateEgressGatewayStatus(ctx, egressGateway, metav1.ConditionTrue, "Programmed", "EgressGateway is programmed and ready"); err != nil {
		logger.Error(err, "failed to update EgressGateway status")
	}

	// Update HTTPRoute statuses
	for httpRouteKey, parentStatuses := range httpRouteStatuses {
		if err := c.updateHTTPRouteStatus(ctx, httpRouteKey, parentStatuses); err != nil {
			logger.Error(err, "failed to update httproute status", "httproute", httpRouteKey)
		}
	}

	return nil
}

// egressGatewayToGateway converts an EgressGateway to a standard Gateway for
// reuse with the existing translator and deployer packages.
// Note: the UID is intentionally not copied so that the deployer does not set
// ownerReferences pointing to a non-existent Gateway resource, which would
// cause the Kubernetes garbage collector to delete the managed resources.
func egressGatewayToGateway(eg *v0alpha0.EgressGateway) *gatewayv1.Gateway {
	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eg.Name,
			Namespace: eg.Namespace,
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: eg.Spec.GatewayClassName,
			Listeners:        make([]gatewayv1.Listener, len(eg.Spec.Listeners)),
			Infrastructure:   eg.Spec.Infrastructure,
		},
	}
	for i, el := range eg.Spec.Listeners {
		gw.Spec.Listeners[i] = gatewayv1.Listener{
			Name:          el.Name,
			Port:          el.Port,
			Protocol:      el.Protocol,
			TLS:           el.TLS,
			AllowedRoutes: el.AllowedRoutes,
			// Hostname intentionally nil — egress has no frontend hostname.
		}
	}
	return gw
}

// updateEgressGatewayStatus updates the EgressGateway status with conditions.
func (c *controller) updateEgressGatewayStatus(ctx context.Context, eg *v0alpha0.EgressGateway, status metav1.ConditionStatus, reason, message string) error {
	egCopy := eg.DeepCopy()

	apimeta.SetStatusCondition(&egCopy.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             "Accepted",
		Message:            "EgressGateway configuration is valid",
		LastTransitionTime: metav1.Now(),
	})

	apimeta.SetStatusCondition(&egCopy.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionProgrammed),
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})

	_, err := c.aigateway.client.AinetworkingV0alpha0().EgressGateways(eg.Namespace).UpdateStatus(ctx, egCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update EgressGateway status: %w", err)
	}

	return nil
}

// updateHTTPRouteStatus updates the HTTPRoute status with the given parent statuses.
func (c *controller) updateHTTPRouteStatus(ctx context.Context, httpRouteKey types.NamespacedName, parentStatuses []gatewayv1.RouteParentStatus) error {
	httpRoute, err := c.gateway.httpRouteLister.HTTPRoutes(httpRouteKey.Namespace).Get(httpRouteKey.Name)
	if err != nil {
		return fmt.Errorf("failed to get httproute: %w", err)
	}

	httpRouteCopy := httpRoute.DeepCopy()
	httpRouteCopy.Status.Parents = parentStatuses

	_, err = c.gateway.client.GatewayV1().HTTPRoutes(httpRoute.Namespace).UpdateStatus(ctx, httpRouteCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update httproute status: %w", err)
	}

	return nil
}
