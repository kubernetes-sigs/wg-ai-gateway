package envoy

import (
	"fmt"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	transport_socketsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	aigatewayv0alpha0 "sigs.k8s.io/wg-ai-gateway/api/v0alpha0"
	"sigs.k8s.io/wg-ai-gateway/pkg/constants"
)

// buildClustersFromBackends creates Envoy clusters from Backend resources
// Creates one cluster per port declared on each backend
func (t *translator) buildClustersFromBackends(backends []*aigatewayv0alpha0.Backend) ([]*clusterv3.Cluster, error) {
	var clusters []*clusterv3.Cluster

	for _, backend := range backends {
		// Determine the ports to create clusters for
		var ports []uint32
		if len(backend.Spec.Destination.Ports) > 0 {
			// Use explicitly declared ports
			for _, port := range backend.Spec.Destination.Ports {
				ports = append(ports, port.Number)
			}
		} else {
			// Use default ports based on backend type
			if backend.Spec.Destination.Type == aigatewayv0alpha0.BackendTypeFqdn {
				ports = []uint32{80} // Default HTTP port for FQDN
			} else {
				ports = []uint32{80} // Default port for K8s services too
			}
		}

		// Create one cluster per port
		for _, port := range ports {
			clusterName := fmt.Sprintf(constants.ClusterNameFormat, backend.Namespace, backend.Name)
			if port != 80 && port != 443 {
				// Add port suffix for non-standard ports to avoid naming conflicts
				clusterName = fmt.Sprintf("%s-%d", clusterName, port)
			}

			cluster := &clusterv3.Cluster{
				Name:           clusterName,
				ConnectTimeout: &durationpb.Duration{Seconds: 5},
			}

			// Configure the cluster based on the backend type
			switch backend.Spec.Destination.Type {
			case aigatewayv0alpha0.BackendTypeFqdn:
				// For FQDN backends, use DNS discovery
				cluster.ClusterDiscoveryType = &clusterv3.Cluster_Type{Type: clusterv3.Cluster_LOGICAL_DNS}
				cluster.DnsLookupFamily = clusterv3.Cluster_V4_ONLY
				if backend.Spec.Destination.FQDN != nil {
					cluster.LoadAssignment = t.createClusterLoadAssignment(clusterName, backend.Spec.Destination.FQDN.Hostname, port)
				}

			case aigatewayv0alpha0.BackendTypeKubernetesService:
				// For Kubernetes services, use EDS to get endpoints directly
				cluster.ClusterDiscoveryType = &clusterv3.Cluster_Type{Type: clusterv3.Cluster_EDS}
				cluster.EdsClusterConfig = &clusterv3.Cluster_EdsClusterConfig{
					EdsConfig: &corev3.ConfigSource{
						ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
							Ads: &corev3.AggregatedConfigSource{},
						},
						ResourceApiVersion: resourcev3.DefaultAPIVersion,
					},
					ServiceName: clusterName,
				}
				// No LoadAssignment needed - endpoints will come from EDS

			default:
				return nil, fmt.Errorf("unsupported backend type: %s", backend.Spec.Destination.Type)
			}

			clusters = append(clusters, cluster)
		}
	}

	return clusters, nil
}

// createClusterLoadAssignment creates a cluster load assignment for a given service
func (t *translator) createClusterLoadAssignment(clusterName, serviceHost string, servicePort uint32) *endpointv3.ClusterLoadAssignment {
	return &endpointv3.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*endpointv3.LocalityLbEndpoints{
			{
				LbEndpoints: []*endpointv3.LbEndpoint{
					{
						HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
							Endpoint: &endpointv3.Endpoint{
								Address: &corev3.Address{
									Address: &corev3.Address_SocketAddress{
										SocketAddress: &corev3.SocketAddress{
											Address: serviceHost,
											PortSpecifier: &corev3.SocketAddress_PortValue{
												PortValue: servicePort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// translateListenerToFilterChain creates a filter chain for an Envoy listener
func (t *translator) translateListenerToFilterChain(gateway *gatewayv1.Gateway, listener gatewayv1.Listener, routeName string) (*listenerv3.FilterChain, error) {
	// Create HTTP connection manager filter
	hcm := &hcmv3.HttpConnectionManager{
		CodecType:  hcmv3.HttpConnectionManager_AUTO,
		StatPrefix: fmt.Sprintf("gateway_%s_listener_%s", gateway.Name, listener.Name),
		RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
			Rds: &hcmv3.Rds{
				ConfigSource: &corev3.ConfigSource{
					ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
						Ads: &corev3.AggregatedConfigSource{},
					},
					ResourceApiVersion: resourcev3.DefaultAPIVersion,
				},
				RouteConfigName: routeName,
			},
		},
	}

	// Serialize the HTTP connection manager
	hcmAny, err := anypb.New(hcm)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize HTTP connection manager: %w", err)
	}

	filterChain := &listenerv3.FilterChain{
		Filters: []*listenerv3.Filter{
			{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &listenerv3.Filter_TypedConfig{
					TypedConfig: hcmAny,
				},
			},
		},
	}

	// Handle TLS configuration for HTTPS listeners
	if listener.Protocol == gatewayv1.HTTPSProtocolType {
		// Create basic TLS context
		tlsContext := &transport_socketsv3.DownstreamTlsContext{}

		// Add SNI matching if hostname is specified
		if listener.Hostname != nil {
			filterChain.FilterChainMatch = &listenerv3.FilterChainMatch{
				ServerNames: []string{string(*listener.Hostname)},
			}
		}

		tlsAny, err := anypb.New(tlsContext)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize TLS context: %w", err)
		}

		filterChain.TransportSocket = &corev3.TransportSocket{
			Name: wellknown.TransportSocketTLS,
			ConfigType: &corev3.TransportSocket_TypedConfig{
				TypedConfig: tlsAny,
			},
		}
	}

	return filterChain, nil
}

// createEnvoyAddress creates an Envoy address configuration
func (t *translator) createEnvoyAddress(port uint32) *corev3.Address {
	return &corev3.Address{
		Address: &corev3.Address_SocketAddress{
			SocketAddress: &corev3.SocketAddress{
				Address: "0.0.0.0",
				PortSpecifier: &corev3.SocketAddress_PortValue{
					PortValue: port,
				},
			},
		},
	}
}

// generateEDSFromService creates EDS endpoints for a Kubernetes service using EndpointSlices
func (t *translator) generateEDSFromService(serviceName, serviceNamespace string, servicePort uint32) (*endpointv3.ClusterLoadAssignment, error) {
	// Get EndpointSlices for the service
	selector := labels.Set(map[string]string{
		discoveryv1.LabelServiceName: serviceName,
	}).AsSelector()

	endpointSlices, err := t.endpointSliceLister.EndpointSlices(serviceNamespace).List(selector)
	if err != nil {
		return nil, fmt.Errorf("failed to list EndpointSlices for service %s/%s: %w", serviceNamespace, serviceName, err)
	}

	clusterName := fmt.Sprintf(constants.ClusterNameFormat, serviceNamespace, serviceName)
	if servicePort != 80 && servicePort != 443 {
		clusterName = fmt.Sprintf("%s-%d", clusterName, servicePort)
	}

	var lbEndpoints []*endpointv3.LbEndpoint

	// Iterate through all EndpointSlices for this service
	for _, es := range endpointSlices {
		// Find the port in the EndpointSlice that matches our target port
		var targetPortName string
		for _, port := range es.Ports {
			if port.Port != nil && uint32(*port.Port) == servicePort {
				if port.Name != nil {
					targetPortName = *port.Name
				}
				break
			}
		}

		// Process endpoints in this slice
		for _, endpoint := range es.Endpoints {
			// Skip endpoints that are not ready
			if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
				continue
			}

			// Add each address in the endpoint
			for _, address := range endpoint.Addresses {
				lbEndpoint := &endpointv3.LbEndpoint{
					HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
						Endpoint: &endpointv3.Endpoint{
							Address: &corev3.Address{
								Address: &corev3.Address_SocketAddress{
									SocketAddress: &corev3.SocketAddress{
										Address: address,
										PortSpecifier: &corev3.SocketAddress_PortValue{
											PortValue: servicePort,
										},
									},
								},
							},
						},
					},
				}

				// Set health check configuration if the endpoint has health info
				if endpoint.Conditions.Ready != nil {
					lbEndpoint.HealthStatus = endpointv3.HealthStatus_HEALTHY
				}

				lbEndpoints = append(lbEndpoints, lbEndpoint)
			}
		}
	}

	return &endpointv3.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*endpointv3.LocalityLbEndpoints{
			{
				LbEndpoints: lbEndpoints,
			},
		},
	}, nil
}
