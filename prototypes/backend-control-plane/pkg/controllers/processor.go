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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/api/v0alpha0"
	aigatewayinformers "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/k8s/client/informers/externalversions/api/v0alpha0"
)

func (c *controller) setupProcessorEventHandlers(processorInformer aigatewayinformers.XPayloadProcessorInformer) error {
	_, err := processorInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueGatewaysForProcessor(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.enqueueGatewaysForProcessor(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueGatewaysForProcessor(obj)
		},
	})
	return err
}

// enqueueGatewaysForProcessor finds all Gateways that might be affected by a processor
// change by scanning HTTPRoutes for ExtensionRef filters that reference the processor.
func (c *controller) enqueueGatewaysForProcessor(obj interface{}) {
	processor, ok := obj.(*v0alpha0.XPayloadProcessor)
	if !ok {
		klog.ErrorS(nil, "Expected XPayloadProcessor object", "obj", obj)
		return
	}

	// Find HTTPRoutes that reference this processor via ExtensionRef filters
	httpRoutes, err := c.gateway.httpRouteLister.List(labels.Everything())
	if err != nil {
		klog.ErrorS(err, "Failed to list HTTPRoutes for processor change")
		return
	}

	for _, route := range httpRoutes {
		if !routeReferencesProcessor(route, processor) {
			continue
		}
		// Enqueue all parent Gateways for this route
		for _, parentRef := range route.Spec.ParentRefs {
			namespace := route.Namespace
			if parentRef.Namespace != nil {
				namespace = string(*parentRef.Namespace)
			}
			kind := "Gateway"
			if parentRef.Kind != nil {
				kind = string(*parentRef.Kind)
			}
			if kind != "Gateway" {
				continue
			}
			gatewayKey := types.NamespacedName{
				Namespace: namespace,
				Name:      string(parentRef.Name),
			}.String()

			klog.V(4).InfoS("Enqueuing Gateway due to XPayloadProcessor change",
				"gateway", gatewayKey,
				"processor", types.NamespacedName{Namespace: processor.Namespace, Name: processor.Name})

			c.gatewayqueue.Add(gatewayKey)
		}
	}
}

// routeReferencesProcessor checks if an HTTPRoute has an ExtensionRef filter
// pointing to the given processor.
func routeReferencesProcessor(route *gatewayv1.HTTPRoute, processor *v0alpha0.XPayloadProcessor) bool {
	for _, rule := range route.Spec.Rules {
		for _, filter := range rule.Filters {
			if filter.Type != gatewayv1.HTTPRouteFilterExtensionRef || filter.ExtensionRef == nil {
				continue
			}
			if string(filter.ExtensionRef.Kind) == "XPayloadProcessor" &&
				string(filter.ExtensionRef.Name) == processor.Name {
				// Namespace match: ExtensionRef doesn't have a namespace field,
				// so it's always in the same namespace as the HTTPRoute.
				if route.Namespace == processor.Namespace {
					return true
				}
			}
		}
	}
	return false
}
