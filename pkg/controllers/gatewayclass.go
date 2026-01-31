package controllers

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions/apis/v1"

	"sigs.k8s.io/wg-ai-gateway/pkg/constants"
)

func (c *controller) setupGatewayClassEventHandlers(gatewayClassInformer gatewayinformers.GatewayClassInformer) error {
	_, err := gatewayClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.syncGatewayClass(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(newObj)
			if err == nil {
				c.syncGatewayClass(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.syncGatewayClass(key)
			}
		},
	})
	return err
}

func (c *controller) syncGatewayClass(key string) {
	startTime := time.Now()
	klog.V(2).Infof("Started syncing gatewayclass %q (%v)", key, time.Since(startTime))
	defer func() {
		klog.V(2).Infof("Finished syncing gatewayclass %q (%v)", key, time.Since(startTime))
	}()

	gwc, err := c.gateway.gatewayClassLister.Get(key)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.InfoS("GatewayClass deleted", "gatewayclass", key)
		}
		return
	}

	// We only care about the GatewayClass that matches our controller name.
	if gwc.Spec.ControllerName != constants.EnvoyControllerName {
		klog.V(5).Infof("Ignoring GatewayClass %q with unknown controller name %q", key, gwc.Spec.ControllerName)
		return
	}

	newGwc := gwc.DeepCopy()
	// Set the "Accepted" condition to True and update the observedGeneration.
	meta.SetStatusCondition(&newGwc.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayClassConditionStatusAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.GatewayClassReasonAccepted),
		Message:            fmt.Sprintf("GatewayClass is accepted by the %s controller.", constants.EnvoyControllerName),
		ObservedGeneration: gwc.Generation,
	})

	// Update the GatewayClass status
	if _, err := c.gateway.client.GatewayV1().GatewayClasses().UpdateStatus(context.Background(), newGwc, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("failed to update gatewayclass status: %v", err)
	} else {
		klog.InfoS("GatewayClass status updated", "gatewayclass", key)
	}
}
