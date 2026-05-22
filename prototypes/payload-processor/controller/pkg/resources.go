// Package pkg implements resource generation for agentgateway-compatible xDS.
//
// This file translates Gateway and HTTPRoute resources into the Bind, Listener,
// Route, and Backend resources that the agentgateway data plane expects.
// Scoped to the minimum needed for the PayloadProcessor POC.
package pkg

import (
	"fmt"
	"log/slog"

	"k8s.io/apimachinery/pkg/types"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/agentgateway/agentgateway/api"
)

// GatewayResource wraps an agentgateway Resource with the gateway it belongs to.
type GatewayResource struct {
	Gateway  types.NamespacedName
	Resource *api.Resource
}

func (r GatewayResource) ResourceName() string {
	return fmt.Sprintf("%s/%s/%s", r.Gateway.Namespace, r.Gateway.Name, resourceKey(r.Resource))
}

func resourceKey(r *api.Resource) string {
	switch k := r.Kind.(type) {
	case *api.Resource_Bind:
		return k.Bind.Key
	case *api.Resource_Listener:
		return k.Listener.Key
	case *api.Resource_Route:
		return k.Route.Key
	case *api.Resource_Policy:
		return k.Policy.Key
	default:
		return "unknown"
	}
}

// TranslateGateway converts a Gateway resource into Bind and Listener resources.
func TranslateGateway(gw *gwv1.Gateway) []GatewayResource {
	gwNN := types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name}
	var resources []GatewayResource

	for _, listener := range gw.Spec.Listeners {
		port := uint32(listener.Port)
		bindKey := fmt.Sprintf("%d/%s/%s", port, gw.Namespace, gw.Name)
		listenerKey := fmt.Sprintf("%s/%s.%s", gw.Namespace, gw.Name, listener.Name)

		// Bind — port binding
		bind := &api.Resource{
			Kind: &api.Resource_Bind{
				Bind: &api.Bind{
					Key:            bindKey,
					Port:           port,
					Protocol:       api.Bind_HTTP,
					TunnelProtocol: api.Bind_DIRECT,
				},
			},
		}
		resources = append(resources, GatewayResource{Gateway: gwNN, Resource: bind})

		// Listener — hostname/protocol
		hostname := "*"
		if listener.Hostname != nil {
			hostname = string(*listener.Hostname)
		}
		lis := &api.Resource{
			Kind: &api.Resource_Listener{
				Listener: &api.Listener{
					Key:      listenerKey,
					BindKey:  bindKey,
					Hostname: hostname,
					Protocol: api.Protocol_HTTP,
					Name: &api.ListenerName{
						GatewayNamespace: gw.Namespace,
						GatewayName:      gw.Name,
						ListenerName:     string(listener.Name),
					},
				},
			},
		}
		resources = append(resources, GatewayResource{Gateway: gwNN, Resource: lis})
	}

	slog.Debug("translated Gateway", "name", gw.Name, "resources", len(resources))
	return resources
}

// TranslateHTTPRoute converts an HTTPRoute into Route resources with backend references.
func TranslateHTTPRoute(
	route *gwv1.HTTPRoute,
	getGateway func(types.NamespacedName) *gwv1.Gateway,
) []GatewayResource {
	var resources []GatewayResource

	for _, parentRef := range route.Spec.ParentRefs {
		// Resolve the parent gateway
		parentNS := route.Namespace
		if parentRef.Namespace != nil {
			parentNS = string(*parentRef.Namespace)
		}
		gwNN := types.NamespacedName{Namespace: parentNS, Name: string(parentRef.Name)}

		// Find the gateway to determine the listener key
		gw := getGateway(gwNN)
		if gw == nil {
			slog.Warn("parent Gateway not found for HTTPRoute", "route", route.Name, "gateway", gwNN)
			continue
		}

		// Determine which listener this route attaches to
		listenerName := ""
		if parentRef.SectionName != nil {
			listenerName = string(*parentRef.SectionName)
		} else if len(gw.Spec.Listeners) > 0 {
			listenerName = string(gw.Spec.Listeners[0].Name)
		}
		listenerKey := fmt.Sprintf("%s/%s.%s", parentNS, gwNN.Name, listenerName)

		// Translate each rule
		for ruleIdx, rule := range route.Spec.Rules {
			routeKey := fmt.Sprintf("route/%s/%s.%02d.http", route.Namespace, route.Name, ruleIdx)

			// Build matches
			var matches []*api.RouteMatch
			for _, match := range rule.Matches {
				m := &api.RouteMatch{}

				// Path match
				if match.Path != nil {
					pathType := gwv1.PathMatchPathPrefix
					if match.Path.Type != nil {
						pathType = *match.Path.Type
					}
					pathValue := "/"
					if match.Path.Value != nil {
						pathValue = *match.Path.Value
					}
					switch pathType {
					case gwv1.PathMatchPathPrefix:
						m.Path = &api.PathMatch{Kind: &api.PathMatch_PathPrefix{PathPrefix: pathValue}}
					case gwv1.PathMatchExact:
						m.Path = &api.PathMatch{Kind: &api.PathMatch_Exact{Exact: pathValue}}
					}
				}

				// Header matches
				for _, hdr := range match.Headers {
					headerMatch := &api.HeaderMatch{
						Name: string(hdr.Name),
					}
					if hdr.Type != nil && *hdr.Type == gwv1.HeaderMatchRegularExpression {
						headerMatch.Value = &api.HeaderMatch_Regex{Regex: hdr.Value}
					} else {
						headerMatch.Value = &api.HeaderMatch_Exact{Exact: hdr.Value}
					}
					m.Headers = append(m.Headers, headerMatch)
				}

				matches = append(matches, m)
			}

			// Build backends
			var backends []*api.RouteBackend
			for _, backendRef := range rule.BackendRefs {
				weight := int32(1)
				if backendRef.Weight != nil {
					weight = int32(*backendRef.Weight)
				}
				port := uint32(0)
				if backendRef.Port != nil {
					port = uint32(*backendRef.Port)
				}
				ns := route.Namespace
				if backendRef.Namespace != nil {
					ns = string(*backendRef.Namespace)
				}
				name := string(backendRef.Name)
				hostname := fmt.Sprintf("%s.%s.svc.cluster.local", name, ns)

				backends = append(backends, &api.RouteBackend{
					Weight: weight,
					Backend: &api.BackendReference{
						Kind: &api.BackendReference_Service_{
							Service: &api.BackendReference_Service{
								Namespace: ns,
								Hostname:  hostname,
							},
						},
						Port: port,
					},
				})
			}

			r := &api.Resource{
				Kind: &api.Resource_Route{
					Route: &api.Route{
						Key:         routeKey,
						ListenerKey: listenerKey,
						Name: &api.RouteName{
							Kind:      "HTTPRoute",
							Namespace: route.Namespace,
							Name:      route.Name,
						},
						Matches:  matches,
						Backends: backends,
					},
				},
			}
			resources = append(resources, GatewayResource{Gateway: gwNN, Resource: r})
		}
	}

	slog.Debug("translated HTTPRoute", "name", route.Name, "resources", len(resources))
	return resources
}
