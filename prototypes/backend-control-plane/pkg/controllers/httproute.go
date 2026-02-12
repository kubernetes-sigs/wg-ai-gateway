package controllers

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions/apis/v1"
)

func (c *controller) setupHTTPRouteEventHandlers(httpRouteInformer gatewayinformers.HTTPRouteInformer) error {
	_, err := httpRouteInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueHTTPRouteParentGateways(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.enqueueHTTPRouteParentGateways(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueHTTPRouteParentGateways(obj)
		},
	})
	return err
}

func (c *controller) enqueueHTTPRouteParentGateways(obj interface{}) {
	httpRoute, ok := obj.(*gatewayv1.HTTPRoute)
	if !ok {
		klog.ErrorS(nil, "Expected HTTPRoute object", "obj", obj)
		return
	}

	// Enqueue all parent Gateways referenced by this HTTPRoute
	for _, parentRef := range httpRoute.Spec.ParentRefs {
		// Default to same namespace if not specified
		namespace := httpRoute.Namespace
		if parentRef.Namespace != nil {
			namespace = string(*parentRef.Namespace)
		}

		// Default to Gateway kind if not specified
		kind := "Gateway"
		if parentRef.Kind != nil {
			kind = string(*parentRef.Kind)
		}

		// Only handle Gateway resources
		if kind != "Gateway" {
			continue
		}

		gatewayKey := types.NamespacedName{
			Namespace: namespace,
			Name:      string(parentRef.Name),
		}.String()

		klog.V(4).InfoS("Enqueuing Gateway due to HTTPRoute change",
			"gateway", gatewayKey,
			"httproute", types.NamespacedName{Namespace: httpRoute.Namespace, Name: httpRoute.Name})

		c.gatewayqueue.Add(gatewayKey)
	}
}
