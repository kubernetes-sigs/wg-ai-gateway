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

// IMPORTANT: Run "make generate" to regenerate code after modifying this file

package v0alpha0

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gateway "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// XPayloadProcessor defines an external processing step that can inspect and mutate
// HTTP request and response payloads. It is referenced from HTTPRoute rules via
// ExtensionRef filters to add payload-level processing (guardrails, semantic caching,
// content transformation) to routes.
type XPayloadProcessor struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`
	// +required
	Spec XPayloadProcessorSpec `json:"spec"`
	// +optional
	Status XPayloadProcessorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// XPayloadProcessorList contains a list of XPayloadProcessor.
type XPayloadProcessorList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []XPayloadProcessor `json:"items"`
}

// XPayloadProcessorSpec defines the desired state of XPayloadProcessor.
type XPayloadProcessorSpec struct {
	// backendRef identifies the gRPC service implementing the Envoy ext_proc protocol.
	// The service must implement envoy.service.ext_proc.v3.ExternalProcessor.
	// +required
	BackendRef ProcessorBackendRef `json:"backendRef"`

	// processingMode controls which request/response phases are sent to the processor.
	// If not specified, defaults to sending request headers and buffered request body.
	// +optional
	ProcessingMode *ProcessingMode `json:"processingMode,omitempty"`

	// messageTimeout is the timeout for a single message exchange with the processor.
	// Default: 500ms.
	// +optional
	// +kubebuilder:default="500ms"
	MessageTimeout *metav1.Duration `json:"messageTimeout,omitempty"`

	// failureMode determines behavior when the processor is unreachable or returns an error.
	// +required
	// +kubebuilder:default="Closed"
	FailureMode ProcessorFailureMode `json:"failureMode"`
}

// ProcessorBackendRef identifies a gRPC service that implements the ext_proc protocol.
type ProcessorBackendRef struct {
	// name is the name of the backend destination.
	// +required
	Name gateway.ObjectName `json:"name"`
	// namespace is the namespace of the backend. Defaults to the namespace of the
	// referencing XPayloadProcessor.
	// +optional
	Namespace *gateway.Namespace `json:"namespace,omitempty"`
	// kind is the kind of the backend. Defaults to "Service".
	// +optional
	// +kubebuilder:default="Service"
	Kind *gateway.Kind `json:"kind,omitempty"`
	// port is the port of the gRPC service.
	// +required
	Port gateway.PortNumber `json:"port"`
}

// ProcessingMode controls which request/response phases the processor receives.
type ProcessingMode struct {
	// requestHeaders controls whether request headers are sent to the processor.
	// +optional
	// +kubebuilder:default="Send"
	RequestHeaders *HeaderProcessingMode `json:"requestHeaders,omitempty"`
	// requestBody controls how the request body is sent to the processor.
	// +optional
	// +kubebuilder:default="Buffered"
	RequestBody *BodyProcessingMode `json:"requestBody,omitempty"`
	// responseHeaders controls whether response headers are sent to the processor.
	// +optional
	// +kubebuilder:default="Skip"
	ResponseHeaders *HeaderProcessingMode `json:"responseHeaders,omitempty"`
	// responseBody controls how the response body is sent to the processor.
	// +optional
	// +kubebuilder:default="Skip"
	ResponseBody *BodyProcessingMode `json:"responseBody,omitempty"`
}

// HeaderProcessingMode controls whether headers are sent to the processor.
// +kubebuilder:validation:Enum=Send;Skip
type HeaderProcessingMode string

const (
	HeaderProcessingModeSend HeaderProcessingMode = "Send"
	HeaderProcessingModeSkip HeaderProcessingMode = "Skip"
)

// BodyProcessingMode controls how the body is sent to the processor.
// +kubebuilder:validation:Enum=Buffered;Streamed;Skip
type BodyProcessingMode string

const (
	// Buffer the entire body before sending it to the processor.
	BodyProcessingModeBuffered BodyProcessingMode = "Buffered"
	// Stream the body to the processor as it arrives.
	BodyProcessingModeStreamed BodyProcessingMode = "Streamed"
	// Do not send the body to the processor.
	BodyProcessingModeSkip BodyProcessingMode = "Skip"
)

// ProcessorFailureMode determines behavior when the processor is unavailable.
// +kubebuilder:validation:Enum=Open;Closed
type ProcessorFailureMode string

const (
	// Allow the request through when the processor is unavailable.
	ProcessorFailureModeOpen ProcessorFailureMode = "Open"
	// Reject the request when the processor is unavailable.
	ProcessorFailureModeClosed ProcessorFailureMode = "Closed"
)

// XPayloadProcessorStatus defines the observed state of XPayloadProcessor.
type XPayloadProcessorStatus struct {
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
