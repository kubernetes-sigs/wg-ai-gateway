package v0alpha0

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Inlined from agentgateway shared types to avoid external dependency.

// CELExpression is a string containing a CEL expression.
//
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=1024
// +k8s:deepcopy-gen=false
type CELExpression string

// LocalPolicyTargetReferenceWithSectionName extends LocalPolicyTargetReference
// with an optional SectionName field.
type LocalPolicyTargetReferenceWithSectionName struct {
	gwv1.LocalPolicyTargetReference `json:",inline"`
	// +optional
	SectionName *gwv1.SectionName `json:"sectionName,omitempty"`
}

// +kubebuilder:rbac:groups=ainetworking.x-k8s.io,resources=payloadprocessors,verbs=get;list;watch
// +kubebuilder:rbac:groups=ainetworking.x-k8s.io,resources=payloadprocessors/status,verbs=get;update;patch

// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".spec.phase",description="Processing phase"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=ainetworking,shortName=pp
// +kubebuilder:subresource:status
type PayloadProcessor struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of PayloadProcessor.
	// +required
	Spec PayloadProcessorSpec `json:"spec"`

	// status defines the current state of PayloadProcessor.
	// +optional
	Status gwv1.PolicyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true
type PayloadProcessorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PayloadProcessor `json:"items"`
}

// +kubebuilder:validation:XValidation:rule="self.phase == 'PreRouting' ? self.targetRef.kind in ['Gateway', 'ListenerSet'] : true",message="phase PreRouting requires targetRef kind to be Gateway or ListenerSet"
// +kubebuilder:validation:XValidation:rule="self.phase == 'PostRouting' ? self.targetRef.kind in ['Gateway', 'ListenerSet', 'HTTPRoute'] : true",message="phase PostRouting requires targetRef kind to be Gateway, ListenerSet, or HTTPRoute"
type PayloadProcessorSpec struct {
	// targetRef identifies the resource this processor attaches to.
	// For PreRouting phase, must be a Gateway or ListenerSet.
	// For PostRouting phase, can also be an HTTPRoute.
	// +required
	TargetRef LocalPolicyTargetReferenceWithSectionName `json:"targetRef"`

	// phase determines when this processor runs relative to route selection.
	// PreRouting runs before route matching (useful for body-based routing).
	// PostRouting runs after route matching (useful for guardrails, transformation).
	// +required
	// +kubebuilder:validation:Enum=PreRouting;PostRouting
	Phase ProcessorPhase `json:"phase"`

	// processors is an ordered list of processing steps.
	// They are executed sequentially in the order specified.
	// +required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +listType=map
	// +listMapKey=name
	Processors []ProcessorEntry `json:"processors"`
}

// +kubebuilder:validation:Enum=PreRouting;PostRouting
type ProcessorPhase string

const (
	ProcessorPhasePreRouting  ProcessorPhase = "PreRouting"
	ProcessorPhasePostRouting ProcessorPhase = "PostRouting"
)

// +kubebuilder:validation:XValidation:rule="self.type == 'InProcess' ? has(self.inProcess) : true",message="inProcess config is required when type is InProcess"
// +kubebuilder:validation:XValidation:rule="self.type == 'ExtProc' ? has(self.extProc) : true",message="extProc config is required when type is ExtProc"
// +kubebuilder:validation:XValidation:rule="self.type == 'InProcess' ? !has(self.extProc) : true",message="extProc must not be set when type is InProcess"
// +kubebuilder:validation:XValidation:rule="self.type == 'ExtProc' ? !has(self.inProcess) : true",message="inProcess must not be set when type is ExtProc"
type ProcessorEntry struct {
	// name is a unique identifier for this processor within the pipeline.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// type specifies how the processor executes.
	// InProcess runs CEL expressions directly in the gateway.
	// ExtProc calls an external gRPC service (not yet implemented).
	// +required
	// +kubebuilder:validation:Enum=InProcess;ExtProccess
	Type ProcessorType `json:"type"`

	// failureMode determines what happens if this processor fails.
	// FailClosed rejects the request on failure (default).
	// FailOpen allows the request to continue on failure.
	// +optional
	// +kubebuilder:default=FailClosed
	// +kubebuilder:validation:Enum=FailClosed;FailOpen
	FailureMode FailureMode `json:"failureMode,omitempty"`

	// timeout is the maximum duration this processor may run.
	// +optional
	// +kubebuilder:validation:XValidation:rule="duration(self) >= duration('10ms')",message="timeout must be at least 10ms"
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// inProcess configures in-process CEL-based payload processing.
	// Required when type is InProcess.
	// +optional
	InProcess *InProcessConfig `json:"inProcess,omitempty"`

	// extProcess configures external processor communication.
	// Required when type is ExtProcess. Not yet implemented.
	// +optional
	ExtProcess *ExtProcessConfig `json:"extProcess,omitempty"`
}

// +kubebuilder:validation:Enum=InProcess;ExtProcess
type ProcessorType string

const (
	ProcessorTypeInProcess  ProcessorType = "InProcess"
	ProcessorTypeExtProcess ProcessorType = "ExtProcess"
)

// +kubebuilder:validation:Enum=FailClosed;FailOpen
type FailureMode string

const (
	FailureModeClosed FailureMode = "FailClosed"
	FailureModeOpen   FailureMode = "FailOpen"
)

// InProcessConfig configures payload processing that runs directly
// in the gateway process using CEL expressions.
//
// +kubebuilder:validation:AtLeastOneFieldSet
type InProcessConfig struct {
	// request defines how to process the request payload.
	// +optional
	Request InProcessTransform `json:"request"`

	// response defines how to process the response payload.
	// +optional
	Response InProcessTransform `json:"response"`
}

// InProcessTransform defines header and body mutations using CEL expressions.
// CEL expressions can access request.body via the json() function,
// e.g. json(request.body).model
//
// +kubebuilder:validation:AtLeastOneFieldSet
type InProcessTransform struct {
	// setHeaders is a list of headers to set (overwrite if existing).
	// The value is a CEL expression.
	//
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +optional
	SetHeaders []HeaderTransformation `json:"setHeaders,omitempty"`

	// removeHeaders is a list of header names to remove.
	//
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +optional
	RemoveHeaders []HeaderName `json:"removeHeaders,omitempty"`

	// setBodyFields is a list of JSON body fields to set (create or overwrite).
	// The field name is a JSONPath expression identifying the target field
	// within the JSON body, and the value is a CEL expression that resolves to
	// the value assigned to that field. When the body is mutated, the
	// Content-Length header is updated automatically by the data plane.
	//
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +optional
	SetBodyFields []BodyFieldTransformation `json:"setBodyFields,omitempty"`

	// removeBodyFields is a list of JSON body fields to remove. Each entry's
	// name is a JSONPath expression identifying the field to remove from the
	// body. Unlike setBodyFields, only the name (path) is interpreted; there is
	// no value expression. When the body is mutated, the Content-Length header
	// is updated automatically by the data plane.
	//
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +optional
	RemoveBodyFields []BodyFieldRemoval `json:"removeBodyFields,omitempty"`
}

// An HTTP Header Name.
//
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=256
// +kubebuilder:validation:Pattern=`^[A-Za-z0-9!#$%&'*+\-.^_\x60|~]+$`
// +k8s:deepcopy-gen=false
type HeaderName string

// JSONPath is a JSONPath expression identifying a field within a JSON body,
// e.g. '$.stream' or '$.stream_options.include_usage'.
//
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=1024
// +k8s:deepcopy-gen=false
type JSONPath string

// HeaderTransformation sets a header to a CEL-evaluated value.
type HeaderTransformation struct {
	// name is the HTTP header name.
	// +required
	Name HeaderName `json:"name"`

	// value is the CEL expression that produces the header value.
	// Use json(request.body).fieldName to extract from the JSON body.
	// +required
	Value CELExpression `json:"value"`
}

// BodyFieldTransformation sets a JSON body field to a CEL-evaluated value.
// The name is a JSONPath expression identifying the target field within the
// JSON body, and the value is a CEL expression that resolves to the value to
// assign.
type BodyFieldTransformation struct {
	// name is the JSONPath expression identifying the target body field
	// (e.g. '$.stream' or '$.stream_options.include_usage').
	// +required
	Name JSONPath `json:"name"`

	// value is the CEL expression that produces the field value.
	// It may reference the request body, e.g. json(request.body).model.
	// +required
	Value CELExpression `json:"value"`
}

// BodyFieldRemoval identifies a JSON body field to remove.
type BodyFieldRemoval struct {
	// name is the JSONPath expression identifying the body field to remove
	// (e.g. '$.user_email').
	// +required
	Name JSONPath `json:"name"`
}

// ExtProcessConfig configures communication with an external processor.
// This is defined for future extensibility and is not yet implemented.
type ExtProcessConfig struct {
	// backendRef references the external processor service.
	// +required
	BackendRef gwv1.BackendObjectReference `json:"backendRef"`
}
