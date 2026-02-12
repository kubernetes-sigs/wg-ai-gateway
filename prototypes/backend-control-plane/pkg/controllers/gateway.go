package controllers

import (
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions/apis/v1"
)

func (c *controller) setupGatewayEventHandlers(gatewayInformer gatewayinformers.GatewayInformer) error {
	_, err := gatewayInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.enqueueGateway(obj, key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(newObj)
			if err == nil {
				c.enqueueGateway(newObj, key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.enqueueGateway(obj, key)
			}
		},
	})
	return err
}

func (c *controller) enqueueGateway(obj interface{}, key string) {
	gateway, ok := obj.(*gatewayv1.Gateway)
	if !ok {
		klog.ErrorS(nil, "Expected Gateway object", "obj", obj)
		return
	}

	// Only process Gateways that use our GatewayClass
	if gateway.Spec.GatewayClassName != "wg-ai-gateway" {
		klog.V(5).InfoS("Ignoring Gateway with different GatewayClass",
			"gateway", key,
			"gatewayClassName", gateway.Spec.GatewayClassName)
		return
	}

	klog.V(4).InfoS("Enqueuing Gateway", "gateway", key)
	c.gatewayqueue.Add(key)
}
