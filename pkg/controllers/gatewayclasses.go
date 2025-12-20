package controllers

import (
	"istio.io/istio/pkg/kube/krt"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type GatewayClass struct {
	Name       string
	Controller gatewayv1.GatewayController
}

func (g GatewayClass) ResourceName() string {
	return g.Name
}

func GatewayClassesCollection(gatewayClasses krt.Collection[*gatewayv1.GatewayClass], opts krt.OptionsBuilder) (
	krt.StatusCollection[*gatewayv1.GatewayClass, gatewayv1.GatewayClassStatus],
	krt.Collection[GatewayClass],
) {
	return krt.NewStatusCollection(gatewayClasses, func(ctx krt.HandlerContext, gwc *gatewayv1.GatewayClass) (*gatewayv1.GatewayClassStatus, *GatewayClass) {

	}, opts.WithName("GatewayClasses")...)
}
