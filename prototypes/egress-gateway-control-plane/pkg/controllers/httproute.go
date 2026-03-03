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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions/apis/v1"

	v0alpha0 "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/api/v0alpha0"
)

func (c *controller) setupHTTPRouteEventHandlers(httpRouteInformer gatewayinformers.HTTPRouteInformer) error {
	_, err := httpRouteInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueHTTPRouteParentEgressGateways(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.enqueueHTTPRouteParentEgressGateways(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueHTTPRouteParentEgressGateways(obj)
		},
	})
	return err
}

func (c *controller) enqueueHTTPRouteParentEgressGateways(obj interface{}) {
	httpRoute, ok := obj.(*gatewayv1.HTTPRoute)
	if !ok {
		klog.ErrorS(nil, "Expected HTTPRoute object", "obj", obj)
		return
	}

	for _, parentRef := range httpRoute.Spec.ParentRefs {
		// Only handle EgressGateway parents
		if parentRef.Kind == nil || string(*parentRef.Kind) != "EgressGateway" {
			continue
		}
		if parentRef.Group == nil || string(*parentRef.Group) != v0alpha0.GroupName {
			continue
		}

		namespace := httpRoute.Namespace
		if parentRef.Namespace != nil {
			namespace = string(*parentRef.Namespace)
		}

		egressGatewayKey := types.NamespacedName{
			Namespace: namespace,
			Name:      string(parentRef.Name),
		}.String()

		klog.V(4).InfoS("Enqueuing EgressGateway due to HTTPRoute change",
			"egressgateway", egressGatewayKey,
			"httproute", types.NamespacedName{Namespace: httpRoute.Namespace, Name: httpRoute.Name})

		c.gatewayqueue.Add(egressGatewayKey)
	}
}
