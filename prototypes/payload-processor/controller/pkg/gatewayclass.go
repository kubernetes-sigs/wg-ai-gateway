// Package pkg implements GatewayClass reconciliation for the agentgateway controller.
//
// When a GatewayClass with our controller name is created, we accept it by
// setting the Accepted condition to True. This mirrors how the agentgateway
// project's controller handles GatewayClass acceptance.
package pkg

import (
	"context"
	"fmt"
	"log/slog"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	deployer "sigs.k8s.io/wg-ai-gateway/prototypes/payload-processor/controller/pkg/deployer/agentgateway"
)

// EnsureGatewayClass creates or updates the agentgateway GatewayClass.
func EnsureGatewayClass(ctx context.Context, client gatewayclient.Interface) error {
	gwcClient := client.GatewayV1().GatewayClasses()

	existing, err := gwcClient.Get(ctx, deployer.AgentGatewayClassName, metav1.GetOptions{})
	if err == nil {
		// Already exists — update status if needed
		return updateGatewayClassStatus(ctx, client, existing)
	}

	// Create the GatewayClass
	gwc := &gwv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: deployer.AgentGatewayClassName,
		},
		Spec: gwv1.GatewayClassSpec{
			ControllerName: gwv1.GatewayController(deployer.AgentGatewayControllerName),
		},
	}

	created, err := gwcClient.Create(ctx, gwc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create GatewayClass %s: %w", deployer.AgentGatewayClassName, err)
	}

	slog.Info("created GatewayClass", "name", deployer.AgentGatewayClassName)
	return updateGatewayClassStatus(ctx, client, created)
}

func updateGatewayClassStatus(ctx context.Context, client gatewayclient.Interface, gwc *gwv1.GatewayClass) error {
	gwcCopy := gwc.DeepCopy()
	meta.SetStatusCondition(&gwcCopy.Status.Conditions, metav1.Condition{
		Type:               string(gwv1.GatewayClassConditionStatusAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(gwv1.GatewayClassReasonAccepted),
		Message:            fmt.Sprintf("GatewayClass is accepted by the %s controller.", deployer.AgentGatewayControllerName),
		ObservedGeneration: gwc.Generation,
	})

	_, err := client.GatewayV1().GatewayClasses().UpdateStatus(ctx, gwcCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update GatewayClass status: %w", err)
	}

	slog.Info("GatewayClass status updated", "name", gwc.Name)
	return nil
}

// ReconcileGatewayClassOnChange re-accepts GatewayClass when it changes.
func ReconcileGatewayClassOnChange(ctx context.Context, client gatewayclient.Interface, gwc *gwv1.GatewayClass, deleted bool) {
	if gwc == nil {
		return
	}
	if string(gwc.Spec.ControllerName) != deployer.AgentGatewayControllerName {
		return
	}

	if deleted {
		slog.Info("GatewayClass deleted, will recreate", "name", gwc.Name)
		if err := EnsureGatewayClass(ctx, client); err != nil {
			slog.Error("failed to recreate GatewayClass", "error", err)
		}
		return
	}

	if err := updateGatewayClassStatus(ctx, client, gwc); err != nil {
		slog.Error("failed to update GatewayClass status", "name", gwc.Name, "error", err)
	}
}
