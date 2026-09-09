package envoy

import (
	"testing"
	"time"

	ext_procv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/backend/api/v0alpha0"
)

func TestBuildExtProcHTTPFilters(t *testing.T) {
	tests := []struct {
		name       string
		processors []*v0alpha0.XPayloadProcessor
		wantCount  int
	}{
		{
			name:       "no processors",
			processors: nil,
			wantCount:  0,
		},
		{
			name: "single processor",
			processors: []*v0alpha0.XPayloadProcessor{
				makeProcessor("ns", "guardrail", "guardrail-svc", 50051, v0alpha0.ProcessorFailureModeClosed),
			},
			wantCount: 1,
		},
		{
			name: "multiple processors",
			processors: []*v0alpha0.XPayloadProcessor{
				makeProcessor("ns", "guardrail", "guardrail-svc", 50051, v0alpha0.ProcessorFailureModeClosed),
				makeProcessor("ns", "cache", "cache-svc", 50052, v0alpha0.ProcessorFailureModeOpen),
			},
			wantCount: 2,
		},
		{
			name: "duplicate processors are deduplicated",
			processors: []*v0alpha0.XPayloadProcessor{
				makeProcessor("ns", "guardrail", "guardrail-svc", 50051, v0alpha0.ProcessorFailureModeClosed),
				makeProcessor("ns", "guardrail", "guardrail-svc", 50051, v0alpha0.ProcessorFailureModeClosed),
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters := buildExtProcHTTPFilters(tt.processors)
			if len(filters) != tt.wantCount {
				t.Errorf("got %d filters, want %d", len(filters), tt.wantCount)
			}
		})
	}
}

func TestBuildExtProcHTTPFilters_FailureMode(t *testing.T) {
	openProc := makeProcessor("ns", "open-proc", "svc", 50051, v0alpha0.ProcessorFailureModeOpen)
	closedProc := makeProcessor("ns", "closed-proc", "svc", 50052, v0alpha0.ProcessorFailureModeClosed)

	openFilters := buildExtProcHTTPFilters([]*v0alpha0.XPayloadProcessor{openProc})
	closedFilters := buildExtProcHTTPFilters([]*v0alpha0.XPayloadProcessor{closedProc})

	if len(openFilters) != 1 || len(closedFilters) != 1 {
		t.Fatal("expected exactly 1 filter each")
	}

	// Verify filter names include namespace/name for identification
	if openFilters[0].Name != "envoy.filters.http.ext_proc/ns/open-proc" {
		t.Errorf("unexpected filter name: %s", openFilters[0].Name)
	}
	if closedFilters[0].Name != "envoy.filters.http.ext_proc/ns/closed-proc" {
		t.Errorf("unexpected filter name: %s", closedFilters[0].Name)
	}
}

func TestBuildProcessorClusters(t *testing.T) {
	tests := []struct {
		name       string
		processors []*v0alpha0.XPayloadProcessor
		wantCount  int
	}{
		{
			name:       "no processors",
			processors: nil,
			wantCount:  0,
		},
		{
			name: "single processor creates one cluster",
			processors: []*v0alpha0.XPayloadProcessor{
				makeProcessor("ns", "guardrail", "guardrail-svc", 50051, v0alpha0.ProcessorFailureModeClosed),
			},
			wantCount: 1,
		},
		{
			name: "processors sharing a backend create one cluster",
			processors: []*v0alpha0.XPayloadProcessor{
				makeProcessor("ns", "proc-a", "shared-svc", 50051, v0alpha0.ProcessorFailureModeClosed),
				makeProcessor("ns", "proc-b", "shared-svc", 50051, v0alpha0.ProcessorFailureModeOpen),
			},
			wantCount: 1,
		},
		{
			name: "processors with different backends create separate clusters",
			processors: []*v0alpha0.XPayloadProcessor{
				makeProcessor("ns", "proc-a", "svc-a", 50051, v0alpha0.ProcessorFailureModeClosed),
				makeProcessor("ns", "proc-b", "svc-b", 50052, v0alpha0.ProcessorFailureModeOpen),
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusters := buildProcessorClusters(tt.processors)
			if len(clusters) != tt.wantCount {
				t.Errorf("got %d clusters, want %d", len(clusters), tt.wantCount)
			}

			// Verify all clusters have HTTP/2 enabled (required for gRPC)
			for _, cluster := range clusters {
				if cluster.Http2ProtocolOptions == nil {
					t.Errorf("cluster %s missing Http2ProtocolOptions", cluster.Name)
				}
			}
		})
	}
}

func TestTranslateProcessingMode(t *testing.T) {
	t.Run("nil mode returns defaults", func(t *testing.T) {
		pm := translateProcessingMode(nil)
		if pm.RequestHeaderMode != ext_procv3.ProcessingMode_SEND {
			t.Errorf("expected SEND for request headers, got %v", pm.RequestHeaderMode)
		}
		if pm.RequestBodyMode != ext_procv3.ProcessingMode_BUFFERED {
			t.Errorf("expected BUFFERED for request body, got %v", pm.RequestBodyMode)
		}
		if pm.ResponseHeaderMode != ext_procv3.ProcessingMode_SKIP {
			t.Errorf("expected SKIP for response headers, got %v", pm.ResponseHeaderMode)
		}
		if pm.ResponseBodyMode != ext_procv3.ProcessingMode_NONE {
			t.Errorf("expected NONE for response body, got %v", pm.ResponseBodyMode)
		}
	})

	t.Run("explicit streamed mode", func(t *testing.T) {
		streamed := v0alpha0.BodyProcessingModeStreamed
		pm := translateProcessingMode(&v0alpha0.ProcessingMode{
			RequestBody: &streamed,
		})
		if pm.RequestBodyMode != ext_procv3.ProcessingMode_STREAMED {
			t.Errorf("expected STREAMED for request body, got %v", pm.RequestBodyMode)
		}
	})
}

func TestMessageTimeout(t *testing.T) {
	t.Run("default timeout", func(t *testing.T) {
		proc := makeProcessor("ns", "p", "svc", 50051, v0alpha0.ProcessorFailureModeClosed)
		d := messageTimeout(proc)
		if d.AsDuration() != 500*time.Millisecond {
			t.Errorf("expected 500ms default, got %v", d.AsDuration())
		}
	})

	t.Run("custom timeout", func(t *testing.T) {
		proc := makeProcessor("ns", "p", "svc", 50051, v0alpha0.ProcessorFailureModeClosed)
		proc.Spec.MessageTimeout = &metav1.Duration{Duration: 2 * time.Second}
		d := messageTimeout(proc)
		if d.AsDuration() != 2*time.Second {
			t.Errorf("expected 2s, got %v", d.AsDuration())
		}
	})
}

func makeProcessor(namespace, name, svcName string, port int32, failureMode v0alpha0.ProcessorFailureMode) *v0alpha0.XPayloadProcessor {
	return &v0alpha0.XPayloadProcessor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v0alpha0.XPayloadProcessorSpec{
			BackendRef: v0alpha0.ProcessorBackendRef{
				Name: gatewayv1.ObjectName(svcName),
				Port: gatewayv1.PortNumber(port),
			},
			FailureMode: failureMode,
		},
	}
}
