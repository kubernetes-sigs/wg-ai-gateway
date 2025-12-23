package envoy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"sigs.k8s.io/wg-ai-gateway/pkg/constants"
	"sigs.k8s.io/wg-ai-gateway/pkg/schema/gvk"
	aigvr "sigs.k8s.io/wg-ai-gateway/pkg/schema/gvr"
)

type object interface {
	metav1.Object
	runtime.Object
}

// Patcher is a function that abstracts patching logic. This is largely because client-go fakes do not handle patching
type patcher func(gvr schema.GroupVersionResource, name string, namespace string, data []byte, subresources ...string) error

type Deployer interface {
	Deploy(ctx context.Context) error
	NodeID() string
}

type listers struct {
	configMapLister      corev1listers.ConfigMapLister
	serviceAccountLister corev1listers.ServiceAccountLister
	serviceLister        corev1listers.ServiceLister
	deploymentLister     appsv1listers.DeploymentLister
}

type deployer struct {
	kubeClient kubernetes.Interface
	listers    *listers
	gateway    *gatewayv1.Gateway
	patcher    patcher
	nodeID     string
	image      string
	namespace  string
}

func NewDeployer(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	gateway *gatewayv1.Gateway,
	image string,
	configMapLister corev1listers.ConfigMapLister,
	serviceAccountLister corev1listers.ServiceAccountLister,
	serviceLister corev1listers.ServiceLister,
	deploymentLister appsv1listers.DeploymentLister,
) Deployer {
	return &deployer{
		gateway:   gateway,
		nodeID:    generateNodeID(gateway.Namespace, gateway.Name),
		image:     image,
		namespace: gateway.Namespace,
		listers: &listers{
			configMapLister:      configMapLister,
			serviceAccountLister: serviceAccountLister,
			serviceLister:        serviceLister,
			deploymentLister:     deploymentLister,
		},
		patcher: func(gvr schema.GroupVersionResource, name string, namespace string, data []byte, subresources ...string) error {
			c := dynamicClient.Resource(gvr).Namespace(namespace)
			t := true
			_, err := c.Patch(context.Background(), name, types.ApplyPatchType, data, metav1.PatchOptions{
				Force:        &t,
				FieldManager: constants.EnvoyControllerName,
			}, subresources...)
			return err
		},
	}
}

func generateNodeID(namespace, name string) string {
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	hash := sha256.Sum256([]byte(namespacedName.String()))
	return fmt.Sprintf(constants.ProxyNameFormat, hex.EncodeToString(hash[:6]))
}

func (d *deployer) NodeID() string {
	return d.nodeID
}

func (d *deployer) Deploy(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues("gateway", klog.KRef(d.gateway.Namespace, d.gateway.Name), "nodeID", d.nodeID)
	ctx = klog.NewContext(ctx, logger)

	manifests, err := renderBaseTemplateForGateway(d.nodeID, d.gateway)
	if err != nil {
		return fmt.Errorf("failed to render base template for gateway %s/%s: %w", d.gateway.Namespace, d.gateway.Name, err)
	}

	return d.apply(ctx, manifests)
}

func (d *deployer) apply(ctx context.Context, manifest []string) error {
	for _, resource := range manifest {
		if err := d.applyOne(ctx, resource); err != nil {
			return fmt.Errorf("error applying resource: %w", err)
		}
	}

	if err := d.waitForGatewayReady(ctx); err != nil {
		return fmt.Errorf("error waiting for gateway %s/%s to be ready: %w", d.gateway.Namespace, d.gateway.Name, err)
	}

	return nil
}

func (d *deployer) applyOne(ctx context.Context, resource string) error {
	logger := klog.FromContext(ctx)
	// First, convert the YAML manifest into an unstructured YAML map
	data := map[string]any{}
	err := yaml.Unmarshal([]byte(resource), &data)
	if err != nil {
		return err
	}

	// Then, use client-go's unstructured type to represent the resource
	unstructuredObj := unstructured.Unstructured{Object: data}
	// set managed label
	clabel := strings.ReplaceAll(constants.EnvoyControllerName, "/", "-")
	err = unstructured.SetNestedField(unstructuredObj.Object, clabel, "metadata", "labels", constants.ManagedGatewayLabel)
	if err != nil {
		return err
	}
	gvr, err := unstructuredToGVR(unstructuredObj)
	if err != nil {
		return err
	}

	canManage, resourceVersion := d.canManage(ctx, gvr, unstructuredObj.GetName(), unstructuredObj.GetNamespace())
	if !canManage {
		logger.V(5).Info("skipping %v/%v/%v, already managed", gvr, unstructuredObj.GetName(), unstructuredObj.GetNamespace())
		return nil
	}
	// Ensure our canManage assertion is not stale
	unstructuredObj.SetResourceVersion(resourceVersion)

	j, err := json.Marshal(unstructuredObj.Object)
	if err != nil {
		return err
	}
	logger.V(5).Info("applying %v", string(j))
	if err := d.patcher(gvr, unstructuredObj.GetName(), unstructuredObj.GetNamespace(), j); err != nil {
		return fmt.Errorf("patch %v/%v/%v: %v", unstructuredObj.GroupVersionKind(), unstructuredObj.GetNamespace(), unstructuredObj.GetName(), err)
	}

	return nil
}

func (d *deployer) waitForGatewayReady(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		err := d.waitForDeploymenAvailable(ctx)
		if err != nil {
			runtimeutil.HandleError(fmt.Errorf("error waiting for deployment available: %w", err))
		}
	})
	wg.Go(func() {
		err := d.waitForServiceReady(ctx)
		if err != nil {
			runtimeutil.HandleError(fmt.Errorf("error waiting for service ready: %w", err))
		}
	})
	wg.Wait()
	logger.Info("gateway resources are ready!")
	return nil
}

func (d *deployer) waitForDeploymenAvailable(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	logger.Info("Waiting for envoy deployment to be available...")
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
		dep, err := d.kubeClient.AppsV1().Deployments(d.namespace).Get(ctx, d.nodeID, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, cond := range dep.Status.Conditions {
			if cond.Type == appsv1.DeploymentAvailable && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("waiting for envoy deployment %s to be available: %w", d.nodeID, err)
	}
	return nil
}

func (d *deployer) waitForServiceReady(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	logger.Info("Waiting for envoy service to be ready...")
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
		svc, err := d.kubeClient.CoreV1().Services(d.namespace).Get(ctx, d.nodeID, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if svc.Spec.ClusterIP != "" {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("waiting for envoy service %s to be ready: %w", d.nodeID, err)
	}
	return nil
}

// unstructuredToGVR extracts the GVR of an unstructured resource. This is useful when using dynamic
// clients.
func unstructuredToGVR(u unstructured.Unstructured) (schema.GroupVersionResource, error) {
	res := schema.GroupVersionResource{}
	gv, err := schema.ParseGroupVersion(u.GetAPIVersion())
	if err != nil {
		return res, err
	}

	gk := gvk.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    u.GetKind(),
	}
	found, ok := gvk.ToGVR(gk)
	if !ok {
		return res, fmt.Errorf("unknown gvk: %v", gk)
	}
	return found, nil
}

// canManage checks if a resource we are about to write should be managed by us. If the resource already exists
// but does not have the ManagedGatewayLabel, we won't overwrite it.
// This ensures we don't accidentally take over some resource we weren't supposed to, which could cause outages.
// Note K8s doesn't have a perfect way to "conditionally SSA", but its close enough (https://github.com/kubernetes/kubernetes/issues/116156).
func (d *deployer) canManage(ctx context.Context, gvr schema.GroupVersionResource, name, namespace string) (bool, string) {
	obj, known := d.getGVRInstance(ctx, gvr, name, namespace)
	if !known {
		// Even though we don't know what it is, allow users to put the resource. We won't be able to
		// protect against overwrites though.
		return true, ""
	}

	if obj == nil {
		// no object, we can manage it
		return true, ""
	}
	_, managed := obj.GetLabels()[constants.ManagedGatewayLabel]
	// If object already exists, we can only manage it if it has the label
	return managed, obj.GetResourceVersion()
}

// Returns the object if it exists as well as well as a bool indicating if the gvr was known to us at all
func (d *deployer) getGVRInstance(ctx context.Context, gvr schema.GroupVersionResource, name, namespace string) (object, bool) {
	logger := klog.FromContext(ctx)
	// N.B the only possible error for these getters is not found
	var obj object
	var err error
	switch gvr {
	case aigvr.ConfigMap:
		obj, err = d.listers.configMapLister.ConfigMaps(namespace).Get(name)
	case aigvr.ServiceAccount:
		obj, err = d.listers.serviceAccountLister.ServiceAccounts(namespace).Get(name)
	case aigvr.Service:
		obj, err = d.listers.serviceLister.Services(namespace).Get(name)
	case aigvr.Deployment:
		obj, err = d.listers.deploymentLister.Deployments(namespace).Get(name)
	default:
		logger.V(3).Info("unknown GVR %v", gvr)
		return nil, false
	}

	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, true
		}
		panic(fmt.Sprintf("unexpected error getting %v/%s/%s: %s", gvr, namespace, name, err))
	}

	return obj, true
}

func DeleteGatewayInfra(ctx context.Context, kubeClient kubernetes.Interface, key types.NamespacedName) error {
	nodeID := generateNodeID(key.Namespace, key.Name)
	logger := klog.FromContext(ctx).WithValues("gateway", klog.KRef(key.Namespace, key.Name), "nodeID", nodeID)
	ctx = klog.NewContext(ctx, logger)
	logger.Info("Deleting gateway infrastructure...")

	err := kubeClient.AppsV1().Deployments(key.Namespace).Delete(ctx, nodeID, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting deployment %s/%s: %w", key.Namespace, nodeID, err)
	}

	err = kubeClient.CoreV1().Services(key.Namespace).Delete(ctx, nodeID, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting service %s/%s: %w", key.Namespace, nodeID, err)
	}

	err = kubeClient.CoreV1().ConfigMaps(key.Namespace).Delete(ctx, nodeID, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting configmap %s/%s: %w", key.Namespace, nodeID, err)
	}

	err = kubeClient.CoreV1().ServiceAccounts(key.Namespace).Delete(ctx, nodeID, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting serviceaccount %s/%s: %w", key.Namespace, nodeID, err)
	}

	logger.Info("Gateway infrastructure deleted.")

	// TODO: Consider removing this data from the xDS cache in some way
	return nil
}
