// PayloadProcessor Reference Controller
//
// A standalone controller that watches PayloadProcessor CRDs and
// delivers agentgateway-compatible policies to data plane instances via xDS.
//
// This controller uses:
//   - Standard client-go informers for reactive K8s resource watching
//   - agentgateway API protos for policy representation
//   - A minimal xDS delta server for config delivery
//
// The core translation logic (PayloadProcessor → policies) is self-contained
// in this repository, adapted from agentgateway's PayloadProcessor plugin.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	gatewayclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	ppclient "sigs.k8s.io/wg-ai-gateway/prototypes/payload-processor/api/k8s/client/clientset/versioned"
	controllerpkg "sigs.k8s.io/wg-ai-gateway/prototypes/payload-processor/controller/pkg"
	agwdeployer "sigs.k8s.io/wg-ai-gateway/prototypes/payload-processor/controller/pkg/deployer/agentgateway"
)

var (
	xdsPort           = flag.Int("xds-port", 9978, "xDS gRPC server port")
	agentgatewayImage = flag.String("agentgateway-image", "", "agentgateway proxy image (default: ghcr.io/agentgateway/agentgateway:latest)")
	controllerService = flag.String("controller-service", "", "controller service address for xDS (e.g. controller.namespace:9978)")
)

func main() {
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := run(); err != nil {
		slog.Error("controller failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create K8s client using in-cluster config or kubeconfig
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig for local development
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		restConfig, err = kubeConfig.ClientConfig()
		if err != nil {
			return fmt.Errorf("failed to create K8s rest config: %w", err)
		}
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create K8s client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	gwClient, err := gatewayclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create gateway-api client: %w", err)
	}

	ppClient, err := ppclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create PayloadProcessor client: %w", err)
	}

	// Create the controller
	controller := controllerpkg.NewController(kubeClient, gwClient, ppClient)

	// Create and attach the gateway deployer
	xdsAddr := *controllerService
	if xdsAddr == "" {
		xdsAddr = fmt.Sprintf("payload-processor-controller.ai-gateway-system:%d", *xdsPort)
	}
	deployer := agwdeployer.NewDeployer(kubeClient, dynamicClient, *agentgatewayImage, xdsAddr)
	controller.SetDeployer(deployer)

	// Ensure the agentgateway GatewayClass exists
	if err := controllerpkg.EnsureGatewayClass(ctx, gwClient); err != nil {
		slog.Warn("failed to ensure GatewayClass (may need Gateway API CRDs installed)", "error", err)
	}

	// Create xDS server
	xds := controllerpkg.NewXDSServer()

	// Wire resource changes to xDS updates
	controller.OnResourceChange(func() {
		xds.UpdateResources(controller.AllResources())
		xds.UpdateAddresses(controller.AllAddresses())
	})

	// Start xDS server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *xdsPort))
	if err != nil {
		return fmt.Errorf("failed to listen on xDS port %d: %w", *xdsPort, err)
	}

	errCh := make(chan error, 2)

	go func() {
		if err := xds.Serve(ctx, lis); err != nil {
			errCh <- fmt.Errorf("xDS server error: %w", err)
		}
	}()

	go func() {
		if err := controller.Run(ctx); err != nil {
			errCh <- fmt.Errorf("controller error: %w", err)
		}
	}()

	slog.Info("PayloadProcessor controller started",
		"xds-port", *xdsPort,
	)

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
		return nil
	case err := <-errCh:
		return err
	}
}
