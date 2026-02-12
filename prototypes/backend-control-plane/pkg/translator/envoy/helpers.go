package envoy

import (
	"fmt"
	"time"

	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	fileaccesslogv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	transport_socketsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/pkg/constants"
	"sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/pkg/protoconv"
)

// buildClustersFromBackends creates Envoy clusters from Backend resources
// Creates one cluster per port declared on each backend
func (t *translator) buildClustersFromBackends(backends []RouteBackend) ([]*clusterv3.Cluster, error) {
	var clusters []*clusterv3.Cluster

	for _, backend := range backends {
		// Determine the ports to create clusters for
		var ports []uint32
		if len(backend.Ports) > 0 {
			// Use explicitly declared ports
			for _, port := range backend.Ports {
				ports = append(ports, port.Number)
			}
		} else {
			return nil, fmt.Errorf("backend %s has no ports defined", backend.String())
		}

		// Create one cluster per port
		for _, port := range ports {
			clusterName := fmt.Sprintf(constants.ClusterNameFormat, backend.Source.Namespace, backend.ClusterName(), port)

			cluster := &clusterv3.Cluster{
				Name:           clusterName,
				ConnectTimeout: &durationpb.Duration{Seconds: 5},
			}

			// Configure the cluster based on the backend type
			switch backend.ResolutionType {
			case RouteBackendResolutionTypeDNS:
				// For FQDN backends, use DNS discovery
				cluster.ClusterDiscoveryType = &clusterv3.Cluster_Type{Type: clusterv3.Cluster_LOGICAL_DNS}
				cluster.DnsLookupFamily = clusterv3.Cluster_V4_ONLY
				if backend.Hostname == "" {
					return nil, fmt.Errorf("backend %s has type FQDN but no FQDN configuration", backend.String())
				}
				cluster.LoadAssignment = t.createClusterLoadAssignment(clusterName, backend.Hostname, port)

			case RouteBackendResolutionTypeEDS:
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
func (t *translator) translateListenerToFilterChain(gateway *gatewayv1.Gateway, listener gatewayv1.Listener, routeConfig *routev3.RouteConfiguration) (*listenerv3.FilterChain, error) {
	// Create HTTP connection manager filter
	hcm := &hcmv3.HttpConnectionManager{
		CodecType:  hcmv3.HttpConnectionManager_AUTO,
		StatPrefix: fmt.Sprintf("gateway_%s_listener_%s", gateway.Name, listener.Name),
		RouteSpecifier: &hcmv3.HttpConnectionManager_RouteConfig{
			RouteConfig: routeConfig,
		},
		// Add HTTP filters - router is required for request routing
		HttpFilters: []*hcmv3.HttpFilter{
			{
				Name: wellknown.Router,
				ConfigType: &hcmv3.HttpFilter_TypedConfig{
					TypedConfig: protoconv.MessageToAny(&routerv3.Router{}),
				},
			},
		},
		// Add proper timeout configurations
		RequestTimeout:    durationpb.New(60 * time.Second), // 60s request timeout
		StreamIdleTimeout: durationpb.New(15 * time.Second), // 15s stream idle timeout
		DrainTimeout:      durationpb.New(15 * time.Second), // 15s drain timeout
		// Enable access logging for debugging
		AccessLog: []*accesslogv3.AccessLog{
			{
				Name: wellknown.FileAccessLog,
				ConfigType: &accesslogv3.AccessLog_TypedConfig{
					TypedConfig: func() *anypb.Any {
						fileAccessLog := &fileaccesslogv3.FileAccessLog{
							Path: "/dev/stdout",
						}
						any, _ := anypb.New(fileAccessLog)
						return any
					}(),
				},
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

	clusterName := fmt.Sprintf(constants.ClusterNameFormat, serviceNamespace, serviceName, servicePort)

	var lbEndpoints []*endpointv3.LbEndpoint

	// Iterate through all EndpointSlices for this service
	for _, es := range endpointSlices {
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
					lbEndpoint.HealthStatus = corev3.HealthStatus_HEALTHY
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
