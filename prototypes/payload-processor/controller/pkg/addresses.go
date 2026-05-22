// Package pkg implements Address resource generation for the agentgateway data plane.
//
// The data plane discovers backends via Address resources (TypeURL:
// "type.googleapis.com/istio.workload.Address"). Without these, the data plane
// cannot resolve service hostnames to endpoints for forwarding traffic.
//
// This file generates minimal Service-type Address resources from K8s Services
// and EndpointSlices, providing the VIP-to-endpoints mapping the data plane needs.
package pkg

import (
	"fmt"
	"log/slog"
	"net"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"

	"istio.io/istio/pkg/workloadapi"

	"google.golang.org/protobuf/types/known/anypb"
)

// AddressTypeURL is the xDS type URL for workload addresses.
const AddressTypeURL = "type.googleapis.com/istio.workload.Address"

// AddressResource wraps a serialized Address for xDS delivery.
type AddressResource struct {
	Key       string
	Marshaled *anypb.Any
}

func (a AddressResource) ResourceName() string {
	return a.Key
}

func (a AddressResource) Equals(other AddressResource) bool {
	return a.Key == other.Key
}

// TranslateServiceToAddress converts a K8s Service into an istio workload Address.
func TranslateServiceToAddress(svc *corev1.Service) *AddressResource {
	if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
		return nil
	}

	hostname := fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace)

	// Build ports
	var ports []*workloadapi.Port
	for _, p := range svc.Spec.Ports {
		targetPort := uint32(p.TargetPort.IntValue())
		if targetPort == 0 {
			targetPort = uint32(p.Port)
		}
		ports = append(ports, &workloadapi.Port{
			ServicePort: uint32(p.Port),
			TargetPort:  targetPort,
		})
	}

	// Build VIP address
	clusterIP := net.ParseIP(svc.Spec.ClusterIP)
	var vipAddr []byte
	if clusterIP != nil {
		if v4 := clusterIP.To4(); v4 != nil {
			vipAddr = v4
		} else {
			vipAddr = clusterIP.To16()
		}
	}

	svcProto := &workloadapi.Service{
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Hostname:  hostname,
		Ports:     ports,
	}
	if vipAddr != nil {
		svcProto.Addresses = []*workloadapi.NetworkAddress{
			{
				Network: "",
				Address: vipAddr,
			},
		}
	}

	addr := &workloadapi.Address{
		Type: &workloadapi.Address_Service{
			Service: svcProto,
		},
	}

	marshaled, err := anypb.New(addr)
	if err != nil {
		slog.Error("failed to marshal Address", "service", svc.Name, "error", err)
		return nil
	}

	key := fmt.Sprintf("%s/%s", svc.Namespace, hostname)
	return &AddressResource{
		Key:       key,
		Marshaled: marshaled,
	}
}

// TranslateEndpointSliceToWorkloads converts a K8s EndpointSlice into
// workload Addresses (one per endpoint/pod).
func TranslateEndpointSliceToWorkloads(
	eps *discoveryv1.EndpointSlice,
) []AddressResource {
	svcName, ok := eps.Labels[discoveryv1.LabelServiceName]
	if !ok {
		return nil
	}

	var results []AddressResource

	for _, endpoint := range eps.Endpoints {
		if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
			continue
		}
		for _, addr := range endpoint.Addresses {
			ip := net.ParseIP(addr)
			if ip == nil {
				continue
			}
			var ipBytes []byte
			if v4 := ip.To4(); v4 != nil {
				ipBytes = v4
			} else {
				ipBytes = ip.To16()
			}

			podName := ""
			if endpoint.TargetRef != nil {
				podName = endpoint.TargetRef.Name
			}
			uid := fmt.Sprintf("%s/%s/%s", eps.Namespace, svcName, addr)

			// Build ports from the EndpointSlice
			var ports []*workloadapi.Port
			for _, p := range eps.Ports {
				if p.Port == nil {
					continue
				}
				ports = append(ports, &workloadapi.Port{
					ServicePort: uint32(*p.Port),
					TargetPort:  uint32(*p.Port),
				})
			}

			workload := &workloadapi.Workload{
				Uid:       uid,
				Name:      podName,
				Namespace: eps.Namespace,
				Addresses: [][]byte{ipBytes},
				Services: map[string]*workloadapi.PortList{
					fmt.Sprintf("%s/%s.%s.svc.cluster.local", eps.Namespace, svcName, eps.Namespace): {
						Ports: ports,
					},
				},
				Status: workloadapi.WorkloadStatus_HEALTHY,
			}

			wAddr := &workloadapi.Address{
				Type: &workloadapi.Address_Workload{
					Workload: workload,
				},
			}

			marshaled, err := anypb.New(wAddr)
			if err != nil {
				slog.Error("failed to marshal workload Address", "uid", uid, "error", err)
				continue
			}

			results = append(results, AddressResource{
				Key:       uid,
				Marshaled: marshaled,
			})
		}
	}

	return results
}

// WorkloadKey returns a stable key for endpoint slice lookups.
func WorkloadKey(nn types.NamespacedName) string {
	return nn.String()
}
