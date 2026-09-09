package envoy

import (
	"fmt"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	ext_procv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	ext_proc_servicev3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/protobuf/types/known/durationpb"

	"sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/api/v0alpha0"
	"sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/pkg/protoconv"
)

// Ensure ext_proc_servicev3 import is used (the proto registration is needed for ext_proc).
var _ = ext_proc_servicev3.File_envoy_service_ext_proc_v3_external_processor_proto

// buildExtProcHTTPFilters creates Envoy ext_proc HTTP filters for each XPayloadProcessor.
// These filters are inserted into the HttpConnectionManager filter chain before the Router filter.
func buildExtProcHTTPFilters(processors []*v0alpha0.XPayloadProcessor) []*hcmv3.HttpFilter {
	// Deduplicate processors by namespace/name
	seen := make(map[string]bool)
	var filters []*hcmv3.HttpFilter

	for _, proc := range processors {
		key := fmt.Sprintf("%s/%s", proc.Namespace, proc.Name)
		if seen[key] {
			continue
		}
		seen[key] = true

		extProcConfig := &ext_procv3.ExternalProcessor{
			GrpcService: &corev3.GrpcService{
				TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{
						ClusterName: processorClusterName(proc),
					},
				},
				Timeout: messageTimeout(proc),
			},
			FailureModeAllow: proc.Spec.FailureMode == v0alpha0.ProcessorFailureModeOpen,
			ProcessingMode:   translateProcessingMode(proc.Spec.ProcessingMode),
			MessageTimeout:   messageTimeout(proc),
		}

		filters = append(filters, &hcmv3.HttpFilter{
			Name: fmt.Sprintf("envoy.filters.http.ext_proc/%s/%s", proc.Namespace, proc.Name),
			ConfigType: &hcmv3.HttpFilter_TypedConfig{
				TypedConfig: protoconv.MessageToAny(extProcConfig),
			},
		})
	}

	return filters
}

// buildProcessorClusters creates gRPC-capable Envoy clusters for processor services.
func buildProcessorClusters(processors []*v0alpha0.XPayloadProcessor) []*clusterv3.Cluster {
	seen := make(map[string]bool)
	var clusters []*clusterv3.Cluster

	for _, proc := range processors {
		clusterName := processorClusterName(proc)
		if seen[clusterName] {
			continue
		}
		seen[clusterName] = true

		backendRef := proc.Spec.BackendRef
		namespace := proc.Namespace
		if backendRef.Namespace != nil {
			namespace = string(*backendRef.Namespace)
		}

		hostname := fmt.Sprintf("%s.%s.svc.cluster.local", backendRef.Name, namespace)

		cluster := &clusterv3.Cluster{
			Name:                 clusterName,
			ConnectTimeout:       durationpb.New(5 * time.Second),
			ClusterDiscoveryType: &clusterv3.Cluster_Type{Type: clusterv3.Cluster_LOGICAL_DNS},
			DnsLookupFamily:      clusterv3.Cluster_V4_ONLY,
			// ext_proc requires HTTP/2 for gRPC
			Http2ProtocolOptions: &corev3.Http2ProtocolOptions{},
			LoadAssignment: &endpointv3.ClusterLoadAssignment{
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
													Address: hostname,
													PortSpecifier: &corev3.SocketAddress_PortValue{
														PortValue: uint32(backendRef.Port),
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
			},
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

func processorClusterName(proc *v0alpha0.XPayloadProcessor) string {
	return fmt.Sprintf("ext_proc_%s_%s_%d", proc.Namespace, proc.Spec.BackendRef.Name, proc.Spec.BackendRef.Port)
}

func messageTimeout(proc *v0alpha0.XPayloadProcessor) *durationpb.Duration {
	if proc.Spec.MessageTimeout != nil {
		return durationpb.New(proc.Spec.MessageTimeout.Duration)
	}
	return durationpb.New(500 * time.Millisecond)
}

func translateProcessingMode(mode *v0alpha0.ProcessingMode) *ext_procv3.ProcessingMode {
	if mode == nil {
		// Default: send request headers and buffered request body
		return &ext_procv3.ProcessingMode{
			RequestHeaderMode:  ext_procv3.ProcessingMode_SEND,
			RequestBodyMode:    ext_procv3.ProcessingMode_BUFFERED,
			ResponseHeaderMode: ext_procv3.ProcessingMode_SKIP,
			ResponseBodyMode:   ext_procv3.ProcessingMode_NONE,
		}
	}

	pm := &ext_procv3.ProcessingMode{}

	if mode.RequestHeaders != nil {
		switch *mode.RequestHeaders {
		case v0alpha0.HeaderProcessingModeSend:
			pm.RequestHeaderMode = ext_procv3.ProcessingMode_SEND
		case v0alpha0.HeaderProcessingModeSkip:
			pm.RequestHeaderMode = ext_procv3.ProcessingMode_SKIP
		}
	} else {
		pm.RequestHeaderMode = ext_procv3.ProcessingMode_SEND
	}

	if mode.RequestBody != nil {
		switch *mode.RequestBody {
		case v0alpha0.BodyProcessingModeBuffered:
			pm.RequestBodyMode = ext_procv3.ProcessingMode_BUFFERED
		case v0alpha0.BodyProcessingModeStreamed:
			pm.RequestBodyMode = ext_procv3.ProcessingMode_STREAMED
		case v0alpha0.BodyProcessingModeSkip:
			pm.RequestBodyMode = ext_procv3.ProcessingMode_NONE
		}
	} else {
		pm.RequestBodyMode = ext_procv3.ProcessingMode_BUFFERED
	}

	if mode.ResponseHeaders != nil {
		switch *mode.ResponseHeaders {
		case v0alpha0.HeaderProcessingModeSend:
			pm.ResponseHeaderMode = ext_procv3.ProcessingMode_SEND
		case v0alpha0.HeaderProcessingModeSkip:
			pm.ResponseHeaderMode = ext_procv3.ProcessingMode_SKIP
		}
	} else {
		pm.ResponseHeaderMode = ext_procv3.ProcessingMode_SKIP
	}

	if mode.ResponseBody != nil {
		switch *mode.ResponseBody {
		case v0alpha0.BodyProcessingModeBuffered:
			pm.ResponseBodyMode = ext_procv3.ProcessingMode_BUFFERED
		case v0alpha0.BodyProcessingModeStreamed:
			pm.ResponseBodyMode = ext_procv3.ProcessingMode_STREAMED
		case v0alpha0.BodyProcessingModeSkip:
			pm.ResponseBodyMode = ext_procv3.ProcessingMode_NONE
		}
	} else {
		pm.ResponseBodyMode = ext_procv3.ProcessingMode_NONE
	}

	return pm
}
