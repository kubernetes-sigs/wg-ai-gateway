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

	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	istiogvr "istio.io/istio/pkg/config/schema/gvr"
	"istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/kube/kclient"
	"istio.io/istio/pkg/kube/krt"
	"istio.io/istio/pkg/kube/kubetypes"
	corev1 "k8s.io/api/core/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	aigatewayv0alpha0 "sigs.k8s.io/wg-ai-gateway/api/v0alpha0"
	"sigs.k8s.io/wg-ai-gateway/pkg/schema/gvr"
)

type Controller interface {
	Run(context.Context)
}

type inputs struct {
	namespaces     krt.Collection[*corev1.Namespace]
	gatewayClasses krt.Collection[*gatewayv1.GatewayClass]
	gateways       krt.Collection[*gatewayv1.Gateway]
	httpRoutes     krt.Collection[*gatewayv1.HTTPRoute]
	services       krt.Collection[*corev1.Service]
	backends       krt.Collection[*aigatewayv0alpha0.Backend]
}

type controller struct {
	xdsCache envoycache.SnapshotCache
	stop     chan struct{}
}

func NewController(
	client kube.Client,
	xdsCache envoycache.SnapshotCache,
) Controller {
	stop := make(chan struct{})
	opts := krt.NewOptionsBuilder(stop, "envoycontroller", krt.GlobalDebugHandler)
	filter := kclient.Filter{
		ObjectFilter: client.ObjectFilter(),
	}
	inputs := inputs{
		namespaces:     krt.NewInformer[*corev1.Namespace](client, opts.WithName("informer/Namespaces")...),
		gatewayClasses: krt.WrapClient(kclient.NewDelayedInformer[*gatewayv1.GatewayClass](client, istiogvr.GatewayClass, kubetypes.StandardInformer, filter), opts.WithName("informer/GatewayClasses")...),
		gateways:       krt.WrapClient(kclient.NewDelayedInformer[*gatewayv1.Gateway](client, istiogvr.Gateway, kubetypes.StandardInformer, filter), opts.WithName("informer/Gateways")...),
		httpRoutes:     krt.WrapClient(kclient.NewDelayedInformer[*gatewayv1.HTTPRoute](client, istiogvr.HTTPRoute, kubetypes.StandardInformer, filter), opts.WithName("informer/HTTPRoutes")...),
		services:       krt.NewInformer[*corev1.Service](client, opts.WithName("informer/Services")...),
		backends:       krt.WrapClient(kclient.NewDelayedInformer[*aigatewayv0alpha0.Backend](client, gvr.Backend, kubetypes.StandardInformer, filter), opts.WithName("informer/Backends")...),
	}

	return &controller{
		xdsCache: xdsCache,
		stop:     stop,
	}
}

// TODO: Implement the controller logic
func (c *controller) Run(ctx context.Context) {
	<-ctx.Done()
	close(c.stop)
}
