// Package pkg implements the PayloadProcessor controller logic.
//
// The translation logic is adapted from the agentgateway controller's
// payload_processor_plugin.go — core business logic lives here, not in agentgateway.
package pkg

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/agentgateway/agentgateway/api"

	v0alpha0 "sigs.k8s.io/wg-ai-gateway/prototypes/payload-processor/api/v0alpha0"
)

// AgwPolicy wraps an agentgateway policy with the gateway it belongs to.
// Adapted from agentgateway's plugins.AgwPolicy.
type AgwPolicy struct {
	Gateway *types.NamespacedName
	Policy  *api.Policy
}

func (p AgwPolicy) ResourceName() string {
	return p.Gateway.String() + "/" + p.Policy.Key
}

// TranslatePayloadProcessor converts a PayloadProcessor CRD into AgwPolicy objects.
// Adapted from agentgateway's payload_processor_plugin.go.
func TranslatePayloadProcessor(
	pp *v0alpha0.PayloadProcessor,
	resolveBackend BackendResolver,
) []AgwPolicy {
	policies, err := translatePayloadProcessorPolicies(pp, resolveBackend)
	if err != nil {
		slog.Error("error translating PayloadProcessor", "name", pp.Name, "namespace", pp.Namespace, "error", err)
		return nil
	}

	if len(policies) == 0 {
		return nil
	}

	targetRef := pp.Spec.TargetRef
	targetNamespace := pp.Namespace

	var agwPolicies []AgwPolicy

	// For this prototype, only Gateway targets are supported (PreRouting phase).
	if string(targetRef.Kind) == "Gateway" {
		gatewayNN := types.NamespacedName{
			Namespace: targetNamespace,
			Name:      string(targetRef.Name),
		}
		policyTarget := gatewayPolicyTarget(targetNamespace, string(targetRef.Name))
		for _, policy := range policies {
			policy.Target = policyTarget
			agwPolicies = append(agwPolicies, AgwPolicy{
				Gateway: &gatewayNN,
				Policy:  policy,
			})
		}
	} else {
		slog.Warn("unsupported targetRef kind in prototype", "kind", targetRef.Kind)
	}

	return agwPolicies
}

// translatePayloadProcessorPolicies converts processor entries into api.Policy objects.
func translatePayloadProcessorPolicies(
	pp *v0alpha0.PayloadProcessor,
	resolveBackend BackendResolver,
) ([]*api.Policy, error) {
	var policies []*api.Policy
	var errs []error

	policyName := types.NamespacedName{
		Namespace: pp.Namespace,
		Name:      pp.Name,
	}
	basePolicyName := fmt.Sprintf("%s/%s", pp.Namespace, pp.Name)

	for i, proc := range pp.Spec.Processors {
		switch proc.Type {
		case v0alpha0.ProcessorTypeExtProcess:
			policy, err := translateExtProcessProcessor(proc, i, basePolicyName, policyName, pp.Spec.Phase, resolveBackend, pp.Namespace)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if policy != nil {
				policies = append(policies, policy)
			}

		case v0alpha0.ProcessorTypeInProcess:
			policy, err := translateInProcessProcessor(proc, i, basePolicyName, policyName, pp.Spec.Phase)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if policy != nil {
				policies = append(policies, policy)
			}

		default:
			errs = append(errs, fmt.Errorf("processor %q: unknown type %q", proc.Name, proc.Type))
		}
	}

	return policies, errors.Join(errs...)
}

func translateExtProcessProcessor(
	proc v0alpha0.ProcessorEntry,
	index int,
	basePolicyName string,
	policyName types.NamespacedName,
	phase v0alpha0.ProcessorPhase,
	resolveBackend BackendResolver,
	namespace string,
) (*api.Policy, error) {
	if proc.ExtProcess == nil {
		return nil, fmt.Errorf("processor %q: ExtProcess config required for ExtProcess type", proc.Name)
	}

	be, err := resolveBackend(proc.ExtProcess.BackendRef, namespace)
	if err != nil {
		return nil, fmt.Errorf("processor %q: failed to resolve extProcess backendRef: %w", proc.Name, err)
	}

	failureMode := api.TrafficPolicySpec_ExtProc_FAIL_CLOSED
	if proc.FailureMode == v0alpha0.FailureModeOpen {
		failureMode = api.TrafficPolicySpec_ExtProc_FAIL_OPEN
	}

	return &api.Policy{
		Key:  fmt.Sprintf("%s:%s[%d]:extprocess", basePolicyName, proc.Name, index),
		Name: typedResourceName("PayloadProcessor", policyName),
		Kind: &api.Policy_Traffic{
			Traffic: &api.TrafficPolicySpec{
				Phase: mapPhase(phase),
				Kind: &api.TrafficPolicySpec_ExtProc_{
					ExtProc: &api.TrafficPolicySpec_ExtProc{
						Target:      be,
						FailureMode: failureMode,
					},
				},
			},
		},
	}, nil
}

func translateInProcessProcessor(
	proc v0alpha0.ProcessorEntry,
	index int,
	basePolicyName string,
	policyName types.NamespacedName,
	phase v0alpha0.ProcessorPhase,
) (*api.Policy, error) {
	if proc.InProcess == nil {
		return nil, fmt.Errorf("processor %q: InProcess config required for InProcess type", proc.Name)
	}

	convertedReq := convertTransform(&proc.InProcess.Request)
	convertedResp := convertTransform(&proc.InProcess.Response)
	if convertedReq != nil || convertedResp != nil {
		return &api.Policy{
			Key:  fmt.Sprintf("%s:%s[%d]:payload-processor", basePolicyName, proc.Name, index),
			Name: typedResourceName("PayloadProcessor", policyName),
			Kind: &api.Policy_Traffic{
				Traffic: &api.TrafficPolicySpec{
					Phase: mapPhase(phase),
					Kind: &api.TrafficPolicySpec_Transformation{
						Transformation: &api.TrafficPolicySpec_TransformationPolicy{
							Request:  convertedReq,
							Response: convertedResp},
					},
				},
			},
		}, nil
	}

	return nil, errors.New("request or response was not specified or was invalid")
}

// convertTransform converts InProcessTransform to the agentgateway API format.
// Adapted from agentgateway's convertTransformSpec.
// TODO(jaellio): validate CEL
func convertTransform(in *v0alpha0.InProcessTransform) *api.TrafficPolicySpec_TransformationPolicy_Transform {
	if in == nil {
		return nil
	}
	var t *api.TrafficPolicySpec_TransformationPolicy_Transform

	for _, h := range in.SetHeaders {
		if t == nil {
			t = &api.TrafficPolicySpec_TransformationPolicy_Transform{}
		}
		t.Set = append(t.Set, &api.TrafficPolicySpec_HeaderTransformation{
			Name:       string(h.Name),
			Expression: string(h.Value),
		})
	}

	if len(in.RemoveHeaders) > 0 {
		if t == nil {
			t = &api.TrafficPolicySpec_TransformationPolicy_Transform{}
		}
		for _, r := range in.RemoveHeaders {
			t.Remove = append(t.Remove, string(r))
		}
	}

	if body := composeBodyExpression(in); body != "" {
		if t == nil {
			t = &api.TrafficPolicySpec_TransformationPolicy_Transform{}
		}
		t.Body = &api.TrafficPolicySpec_BodyTransformation{
			Expression: body,
		}
	}

	return t
}

// composeBodyExpression builds a single CEL body-transformation expression from
// the per-field setBodyFields and removeBodyFields operations.
//
// agentgateway models body mutation as one CEL expression whose result replaces
// the entire request/response body (see agentgateway's transformation_cel.rs:
// the policy has a single `body` expression, and if it fails to evaluate the
// body is replaced with an empty one). There is no per-field set/remove in the
// data plane, so the controller composes the field operations into a single
// expression using the documented CEL functions json(), filterKeys(), merge(),
// and toJson():
//
//	toJson(json(request.body)
//	    .filterKeys(k, !(k in ["<removed>", ...]))  // removeBodyFields
//	    .merge({"<set>": <valueCEL>, ...}))          // setBodyFields
//
// Each field name is a JSONPath; this prototype supports root-level fields, so
// the leading "$." is stripped to obtain the top-level JSON key. Each
// setBodyFields value is a CEL expression emitted verbatim (merge runs after
// filterKeys so a field that is both removed and set ends up set). When the body
// changes, the data plane drops the Content-Length header automatically.
//
// Returns an empty string when there are no body mutations.
func composeBodyExpression(in *v0alpha0.InProcessTransform) string {
	if len(in.SetBodyFields) == 0 && len(in.RemoveBodyFields) == 0 {
		return ""
	}

	expr := "json(request.body)"

	if len(in.RemoveBodyFields) > 0 {
		keys := make([]string, 0, len(in.RemoveBodyFields))
		for _, r := range in.RemoveBodyFields {
			keys = append(keys, strconv.Quote(bodyFieldKey(r.Name)))
		}
		expr += fmt.Sprintf(".filterKeys(k, !(k in [%s]))", strings.Join(keys, ", "))
	}

	if len(in.SetBodyFields) > 0 {
		pairs := make([]string, 0, len(in.SetBodyFields))
		for _, f := range in.SetBodyFields {
			pairs = append(pairs, fmt.Sprintf("%s: %s", strconv.Quote(bodyFieldKey(f.Name)), string(f.Value)))
		}
		expr += fmt.Sprintf(".merge({%s})", strings.Join(pairs, ", "))
	}

	return "toJson(" + expr + ")"
}

// bodyFieldKey converts a root-level JSONPath (e.g. "$.stream") into the
// top-level JSON object key (e.g. "stream"). The prototype supports only
// root-level fields, which is sufficient for OpenAI request-shaping use cases
// such as injecting stream and stream_options.
func bodyFieldKey(path v0alpha0.JSONPath) string {
	k := strings.TrimSpace(string(path))
	k = strings.TrimPrefix(k, "$")
	k = strings.TrimPrefix(k, ".")
	return k
}

// BackendResolver resolves a BackendObjectReference to an agentgateway BackendReference.
type BackendResolver func(ref gwv1.BackendObjectReference, defaultNamespace string) (*api.BackendReference, error)

func mapPhase(phase v0alpha0.ProcessorPhase) api.TrafficPolicySpec_PolicyPhase {
	switch phase {
	case v0alpha0.ProcessorPhasePreRouting:
		return api.TrafficPolicySpec_GATEWAY
	case v0alpha0.ProcessorPhasePostRouting:
		return api.TrafficPolicySpec_ROUTE
	default:
		return api.TrafficPolicySpec_ROUTE
	}
}

func typedResourceName(kind string, nn types.NamespacedName) *api.TypedResourceName {
	return &api.TypedResourceName{
		Kind:      kind,
		Namespace: nn.Namespace,
		Name:      nn.Name,
	}
}

func gatewayPolicyTarget(namespace, name string) *api.PolicyTarget {
	return &api.PolicyTarget{
		Kind: &api.PolicyTarget_Gateway{
			Gateway: &api.PolicyTarget_GatewayTarget{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}
