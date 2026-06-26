// Package pkg implements the agentgateway data plane deployer.
//
// When a Gateway resource with gatewayClassName "agentgateway" is created,
// this deployer creates a Deployment and Service to run the agentgateway proxy.
// The architecture mirrors how the agentgateway project's controller deploys
// its data plane: the control plane watches Gateway resources and manages the
// lifecycle of per-gateway data plane instances.
package agentgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// AgentGatewayClassName is the GatewayClass name that triggers data plane deployment.
	AgentGatewayClassName = "agentgateway"

	// AgentGatewayControllerName is the controller name for the agentgateway GatewayClass.
	AgentGatewayControllerName = "agentgateway.dev/payload-processor-controller"

	// DefaultAgentGatewayImage is the default agentgateway proxy image.
	DefaultAgentGatewayImage = "ghcr.io/agentgateway/agentgateway:latest"

	// managedByLabel marks resources managed by this controller.
	managedByLabel = "app.kubernetes.io/managed-by"
	managedByValue = "payload-processor-controller"

	// gatewayNameLabel links deployed resources back to their parent Gateway.
	gatewayNameLabel      = "gateway.networking.k8s.io/gateway-name"
	gatewayNamespaceLabel = "gateway.networking.k8s.io/gateway-namespace"
)

// Deployer manages the lifecycle of agentgateway data plane instances.
type Deployer struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
	image         string
	xdsAddr       string
}

// NewDeployer creates a new agentgateway data plane deployer.
// xdsAddr is the address the data plane should connect to for xDS (e.g. "controller-svc.namespace:9978").
func NewDeployer(client kubernetes.Interface, dynamicClient dynamic.Interface, image, xdsAddr string) *Deployer {
	if image == "" {
		image = DefaultAgentGatewayImage
	}
	return &Deployer{
		client:        client,
		dynamicClient: dynamicClient,
		image:         image,
		xdsAddr:       xdsAddr,
	}
}

// DeployGateway ensures the data plane Deployment and Service exist for the given Gateway.
func (d *Deployer) DeployGateway(ctx context.Context, gw *gwv1.Gateway) error {
	if string(gw.Spec.GatewayClassName) != AgentGatewayClassName {
		return nil
	}

	name := dataplaneName(gw)
	ns := gw.Namespace

	slog.Info("deploying agentgateway data plane", "gateway", gw.Name, "namespace", ns, "name", name)

	manifests, err := renderBaseTemplate(gw, d.image, d.xdsAddr)
	if err != nil {
		return fmt.Errorf("failed to render template for gateway %s/%s: %w", ns, gw.Name, err)
	}

	for i, manifest := range manifests {
		if err := d.applyManifest(ctx, manifest); err != nil {
			return fmt.Errorf("failed to apply manifest %d for gateway %s/%s: %w", i, ns, gw.Name, err)
		}
	}

	slog.Info("deployed agentgateway data plane", "gateway", gw.Name, "namespace", ns, "name", name)
	return nil
}

// DeleteGateway removes the data plane resources for the given Gateway.
func (d *Deployer) DeleteGateway(ctx context.Context, gwNN types.NamespacedName) error {
	name := dataplaneNameFromNN(gwNN)
	ns := gwNN.Namespace

	slog.Info("deleting agentgateway data plane", "gateway", gwNN.Name, "namespace", ns, "name", name)

	err := d.client.AppsV1().Deployments(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting deployment %s/%s: %w", ns, name, err)
	}

	err = d.client.CoreV1().Services(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting service %s/%s: %w", ns, name, err)
	}

	slog.Info("agentgateway data plane deleted", "gateway", gwNN.Name, "namespace", ns)
	return nil
}

// applyManifest applies a single YAML manifest using server-side apply.
func (d *Deployer) applyManifest(ctx context.Context, manifest string) error {
	obj := &unstructured.Unstructured{}
	if err := yamlutil.NewYAMLOrJSONDecoder(
		bytes.NewReader([]byte(manifest)), len(manifest),
	).Decode(obj); err != nil {
		return fmt.Errorf("failed to decode manifest: %w", err)
	}

	gvr, err := gvkToGVR(obj.GroupVersionKind())
	if err != nil {
		return fmt.Errorf("unsupported resource kind %s: %w", obj.GetKind(), err)
	}

	data, err := json.Marshal(obj.Object)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	_, err = d.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace()).Patch(
		ctx,
		obj.GetName(),
		types.ApplyPatchType,
		data,
		metav1.PatchOptions{FieldManager: managedByValue, Force: boolPtr(true)},
	)
	if err != nil {
		return fmt.Errorf("failed to apply %s %s/%s: %w", obj.GetKind(), obj.GetNamespace(), obj.GetName(), err)
	}

	slog.Info("applied resource", "kind", obj.GetKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
	return nil
}

// gvkToGVR maps a GroupVersionKind to a GroupVersionResource for known types.
func gvkToGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	switch gvk.Kind {
	case "Deployment":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, nil
	case "Service":
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}, nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("unknown kind: %s", gvk.Kind)
	}
}

func boolPtr(b bool) *bool { return &b }

// dataplaneName returns the name for data plane resources associated with a Gateway.
func dataplaneName(gw *gwv1.Gateway) string {
	return fmt.Sprintf("agentgateway-%s", gw.Name)
}

func dataplaneNameFromNN(nn types.NamespacedName) string {
	return fmt.Sprintf("agentgateway-%s", nn.Name)
}
