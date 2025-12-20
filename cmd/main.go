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

	"istio.io/istio/pkg/kube"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	wgaiv0alpha0 "sigs.k8s.io/wg-ai-gateway/api/v0alpha0"
	"sigs.k8s.io/wg-ai-gateway/pkg/xds/envoy"
)

var (
	apiServerURL string
	kubeconfig   string
	resyncPeriod time.Duration
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&apiServerURL, "apiserver-url", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.DurationVar(&resyncPeriod, "resync-period", 30*time.Second, "Resync period for informers.")
	// Istio already knows about gateway api, so add our local scheme
	utilruntime.Must(wgaiv0alpha0.Install(kube.IstioScheme))
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	setupSignals(cancel)
	logger := klog.FromContext(ctx)

	config, err := buildKubeConfig(kubeconfig, apiServerURL)
	if err != nil {
		logger.Error(err, "unable to build kubeconfig")
		os.Exit(1)
	}

	// Create the kube client.
	kubeClient, err := kube.NewClient(kube.NewClientConfigForRestConfig(config), "Kubernetes")
	if err != nil {
		logger.Error(err, "unable to create kube client")
		os.Exit(1)
	}
	// Generate the Envoy control plane
	// TODO: Add callbacks and listener
	cp := envoy.NewControlPlane(ctx, nil, nil)

	// Pass the envoy xDS server to the AI Gateway controller
	// so that the latter can notify the former about config changes.
}

func fatal(l *klog.Logger, err error, msg string) {
	if l == nil {
		klog.Error(err, msg)
	} else {
		l.Error(err, msg)
	}
	os.Exit(1)
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
