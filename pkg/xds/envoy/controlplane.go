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

package envoy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"

	envoy_service_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	envoy_service_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	envoy_service_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	envoy_service_route_v3 "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	envoyproxytypes "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	envoylog "github.com/envoyproxy/go-control-plane/pkg/log"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	xdsserver "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"
	"sigs.k8s.io/wg-ai-gateway/pkg/constants"
)

type ControlPlane interface {
	PushXDS(context.Context, string, map[resourcev3.Type][]envoyproxytypes.Resource) error
	Run(context.Context) error
}

type controlPlane struct {
	server xdsserver.Server
	cache  envoycache.SnapshotCache
}

// slogAdapterForEnvoy adapts *slog.Logger to envoylog.Logger interface
type slogAdapterForEnvoy struct {
	logger *slog.Logger
}

// Ensure it implements the interface
var _ envoylog.Logger = (*slogAdapterForEnvoy)(nil)

func (s *slogAdapterForEnvoy) Debugf(format string, args ...any) {
	if s.logger.Enabled(context.Background(), slog.LevelDebug) {
		s.logger.Debug(fmt.Sprintf(format, args...)) //nolint:sloglint // ignore formatting
	}
}

func (s *slogAdapterForEnvoy) Infof(format string, args ...any) {
	if s.logger.Enabled(context.Background(), slog.LevelInfo) {
		s.logger.Info(fmt.Sprintf(format, args...)) //nolint:sloglint // ignore formatting
	}
}

func (s *slogAdapterForEnvoy) Warnf(format string, args ...any) {
	if s.logger.Enabled(context.Background(), slog.LevelWarn) {
		s.logger.Warn(fmt.Sprintf(format, args...)) //nolint:sloglint // ignore formatting
	}
}

func (s *slogAdapterForEnvoy) Errorf(format string, args ...any) {
	if s.logger.Enabled(context.Background(), slog.LevelError) {
		s.logger.Error(fmt.Sprintf(format, args...)) //nolint:sloglint // ignore formatting
	}
}

func NewControlPlane(
	ctx context.Context,
) ControlPlane {
	baseLogger := slog.Default().With("component", "envoy-controlplane")
	envoyLoggerAdapter := &slogAdapterForEnvoy{logger: baseLogger}

	snapshotCache := envoycache.NewSnapshotCache(false, envoycache.IDHash{}, envoyLoggerAdapter)
	xdsServer := xdsserver.NewServer(ctx, snapshotCache, &callbacks{})

	return &controlPlane{
		server: xdsServer,
		cache:  snapshotCache,
	}
}

func (cp *controlPlane) Run(ctx context.Context) error {
	// This is a prototype server, so we don't bother with auth or TLS.
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(math.MaxInt32),
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				grpc_zap.StreamServerInterceptor(zap.NewNop()),
				func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
					slog.Debug("gRPC call", "method", info.FullMethod)
					return handler(srv, ss)
				},
			)),
	}
	grpcServer := grpc.NewServer(opts...)

	// Register reflection
	reflection.Register(grpcServer)

	// Register xDS services
	envoy_service_endpoint_v3.RegisterEndpointDiscoveryServiceServer(grpcServer, cp.server)
	envoy_service_cluster_v3.RegisterClusterDiscoveryServiceServer(grpcServer, cp.server)
	envoy_service_route_v3.RegisterRouteDiscoveryServiceServer(grpcServer, cp.server)
	envoy_service_listener_v3.RegisterListenerDiscoveryServiceServer(grpcServer, cp.server)
	envoy_service_discovery_v3.RegisterAggregatedDiscoveryServiceServer(grpcServer, cp.server)

	// The xDS server listens on a fixed port (15001) on all interfaces.
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", constants.XDSServerPort))
	if err != nil {
		return err
	}

	// Start the server
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			klog.Errorln("Envoy xDS server failed:", err)
		}
	}()

	// Handle graceful shutdown for both servers
	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	return nil
}

// TODO: Take the xDS resources and update the snapshot cache properly so they are pushed to the appropriate envoy proxies (based on the node ID)
func (cp *controlPlane) PushXDS(ctx context.Context, nodeID string, resources map[resourcev3.Type][]envoyproxytypes.Resource) error {
	return errors.New("TODO: implement PushXDS")
}
