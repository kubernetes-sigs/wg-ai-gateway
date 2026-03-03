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
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	v0alpha0 "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/api/v0alpha0"
	aigatewayinformersv0alpha0 "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/k8s/client/informers/externalversions/api/v0alpha0"
)

func (c *controller) setupEgressGatewayEventHandlers(egressGatewayInformer aigatewayinformersv0alpha0.EgressGatewayInformer) error {
	_, err := egressGatewayInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.enqueueEgressGateway(obj, key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(newObj)
			if err == nil {
				c.enqueueEgressGateway(newObj, key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.enqueueEgressGateway(obj, key)
			}
		},
	})
	return err
}

func (c *controller) enqueueEgressGateway(obj interface{}, key string) {
	eg, ok := obj.(*v0alpha0.EgressGateway)
	if !ok {
		klog.ErrorS(nil, "Expected EgressGateway object", "obj", obj)
		return
	}

	if string(eg.Spec.GatewayClassName) != EgressGatewayClassName {
		klog.V(5).InfoS("Ignoring EgressGateway with different GatewayClass",
			"egressgateway", key,
			"gatewayClassName", eg.Spec.GatewayClassName)
		return
	}

	klog.V(4).InfoS("Enqueuing EgressGateway", "egressgateway", key)
	c.gatewayqueue.Add(key)
}
