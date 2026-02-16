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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	gatewayclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewayinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"

	aigatewayclient "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/k8s/client/clientset/versioned"
	aigatewayinformers "sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/k8s/client/informers/externalversions"
	"sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/pkg/controllers"
)

var (
	apiServerURL    string
	kubeconfig      string
	resyncPeriod    time.Duration
	envoyProxyImage string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&apiServerURL, "apiserver-url", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&envoyProxyImage, "envoy-image", "", "The Envoy proxy image to use for deployed proxies.")
	flag.DurationVar(&resyncPeriod, "resync-period", 0, "Resync period for informers. Typically set to zero")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	setupSignals(cancel)
	logger := klog.FromContext(ctx)

	if envoyProxyImage == "" {
		fatal(&logger, nil, "--envoy-image cannot be empty")
	}

	config, err := buildKubeConfig(kubeconfig, apiServerURL)
	if err != nil {
		fatal(&logger, err, "unable to build kubeconfig")
	}

	// Create all of the clients we'll need
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fatal(&logger, err, "unable to create kubernetes client")
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		fatal(&logger, err, "unable to create dynamic client")
	}
	gatewayClient, err := gatewayclient.NewForConfig(config)
	if err != nil {
		fatal(&logger, err, "unable to create gateway api client")
	}
	aigatewayClient, err := aigatewayclient.NewForConfig(config)
	if err != nil {
		fatal(&logger, err, "unable to create ai gateway client")
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	gatewayInformerFactory := gatewayinformers.NewSharedInformerFactory(gatewayClient, resyncPeriod)
	aigatewayInformerFactory := aigatewayinformers.NewSharedInformerFactory(aigatewayClient, resyncPeriod)

	// Pass the envoy xDS server to the AI Gateway controller
	// so that the latter can notify the former about config changes.
	controller, err := controllers.NewController(
		ctx,
		envoyProxyImage,
		kubeClient,
		dynamicClient,
		gatewayClient,
		aigatewayClient,
		kubeInformerFactory,
		gatewayInformerFactory,
		aigatewayInformerFactory,
	)
	if err != nil {
		fatal(&logger, err, "unable to create controller")
	}

	// NOTE: Ensure all informers are registered before starting them
	// all below
	kubeInformerFactory.Start(ctx.Done())
	gatewayInformerFactory.Start(ctx.Done())
	aigatewayInformerFactory.Start(ctx.Done())

	// Start the controller
	if err := controller.Run(ctx); err != nil {
		fatal(&logger, err, "unable to run controller")
	}
}

func fatal(l *klog.Logger, err error, msg string) {
	if l == nil {
		klog.Error(err, msg)
	} else {
		l.Error(err, msg)
	}
	klog.FlushAndExit(klog.ExitFlushTimeout, 1)
}

func buildKubeConfig(kubeconfig, ApiServerURL string) (*rest.Config, error) {
	if kubeconfig == "" && ApiServerURL == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	// If we're running in-cluster, use the in-cluster config
	if kubeconfig == "" && ApiServerURL == "" {
		if config, err := rest.InClusterConfig(); err == nil {
			return config, nil
		}
	}

	// Otherwise, use the kubeconfig file
	return clientcmd.BuildConfigFromFlags(ApiServerURL, kubeconfig)
}

// setupSignals registers for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program is terminated with exit code 1.
func setupSignals(cancelFunc context.CancelFunc) {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-signals
		cancelFunc()
		<-signals
		os.Exit(1) // second signal. Exit directly.
	}()
}
