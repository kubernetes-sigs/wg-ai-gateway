// Package pkg implements a minimal xDS server for delivering PayloadProcessor
// resources to agentgateway data plane instances.
//
// This is a simplified version of agentgateway's krtxds package, providing
// just enough xDS delta protocol support for the prototype.
package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// TypeURL matches what the agentgateway data plane subscribes to.
const ResourceTypeURL = "type.googleapis.com/agentgateway.dev.resource.Resource"

// XDSServer serves agentgateway-compatible xDS resources to connected data planes.
type XDSServer struct {
	discovery.UnimplementedAggregatedDiscoveryServiceServer

	mu          sync.RWMutex
	resources   map[string]*discovery.Resource // scoped name -> resource (ADP_TYPE)
	addresses   map[string]*discovery.Resource // key -> address resource (ADDRESS_TYPE)
	version     uint64
	connections map[string]chan struct{}

	grpcServer *grpc.Server
}

// NewXDSServer creates a new xDS server.
func NewXDSServer() *XDSServer {
	s := &XDSServer{
		resources:   make(map[string]*discovery.Resource),
		addresses:   make(map[string]*discovery.Resource),
		connections: make(map[string]chan struct{}),
	}
	s.grpcServer = grpc.NewServer()
	discovery.RegisterAggregatedDiscoveryServiceServer(s.grpcServer, s)
	return s
}

// UpdateResources replaces the current set of resources and notifies connected clients.
// Resources are scoped per-gateway using the format: namespace/gateway-name/resource-key
func (s *XDSServer) UpdateResources(resources []GatewayResource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++

	newResources := make(map[string]*discovery.Resource)
	for _, r := range resources {
		// Wrap in the Resource envelope and serialize
		anyProto, err := anypb.New(r.Resource)
		if err != nil {
			slog.Error("failed to marshal resource", "key", resourceKey(r.Resource), "error", err)
			continue
		}

		// Per-gateway scoped name: namespace/gateway-name/resource-key
		scopedName := fmt.Sprintf("%s/%s/%s", r.Gateway.Namespace, r.Gateway.Name, resourceKey(r.Resource))
		b, _ := proto.Marshal(anyProto)
		newResources[scopedName] = &discovery.Resource{
			Name:     scopedName,
			Resource: anyProto,
			Version:  fmt.Sprintf("%d-%d", s.version, len(b)),
		}
	}

	s.resources = newResources

	// Notify all connected clients
	for id, ch := range s.connections {
		select {
		case ch <- struct{}{}:
			slog.Debug("notified xDS client", "id", id)
		default:
		}
	}

	slog.Info("xDS resources updated", "count", len(newResources), "version", s.version)
}

// UpdateAddresses replaces the current set of Address resources and notifies clients.
func (s *XDSServer) UpdateAddresses(addrs []AddressResource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++

	newAddresses := make(map[string]*discovery.Resource)
	for _, a := range addrs {
		newAddresses[a.Key] = &discovery.Resource{
			Name:     a.Key,
			Resource: a.Marshaled,
			Version:  fmt.Sprintf("%d", s.version),
		}
	}

	s.addresses = newAddresses

	for _, ch := range s.connections {
		select {
		case ch <- struct{}{}:
		default:
		}
	}

	slog.Info("xDS addresses updated", "count", len(newAddresses), "version", s.version)
}

// DeltaAggregatedResources implements the xDS delta protocol.
func (s *XDSServer) DeltaAggregatedResources(stream discovery.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	clientID := req.GetNode().GetId()
	if clientID == "" {
		clientID = fmt.Sprintf("unknown-%p", stream)
	}
	slog.Info("xDS client connected", "id", clientID, "type", req.TypeUrl)

	notify := make(chan struct{}, 1)
	s.mu.Lock()
	s.connections[clientID] = notify
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.connections, clientID)
		s.mu.Unlock()
		slog.Info("xDS client disconnected", "id", clientID)
	}()

	// Send initial snapshot
	if err := s.sendSnapshot(stream); err != nil {
		return err
	}

	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-notify:
			if err := s.sendSnapshot(stream); err != nil {
				return err
			}
		}
	}
}

func (s *XDSServer) sendSnapshot(stream discovery.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Send Address resources (services + workloads)
	var addressResources []*discovery.Resource
	for _, r := range s.addresses {
		addressResources = append(addressResources, r)
	}
	if len(addressResources) > 0 {
		if err := stream.Send(&discovery.DeltaDiscoveryResponse{
			TypeUrl:   AddressTypeURL,
			Resources: addressResources,
		}); err != nil {
			return err
		}
	}

	// Send ADP resources (binds, listeners, routes, policies)
	var adpResources []*discovery.Resource
	for _, r := range s.resources {
		adpResources = append(adpResources, r)
	}
	return stream.Send(&discovery.DeltaDiscoveryResponse{
		TypeUrl:   ResourceTypeURL,
		Resources: adpResources,
	})
}

// Serve starts the xDS gRPC server on the given listener.
func (s *XDSServer) Serve(ctx context.Context, lis net.Listener) error {
	slog.Info("xDS server listening", "addr", lis.Addr())

	go func() {
		<-ctx.Done()
		s.grpcServer.GracefulStop()
	}()

	return s.grpcServer.Serve(lis)
}
