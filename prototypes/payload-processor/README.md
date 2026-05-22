# PayloadProcessor Prototype

This directory contains a prototype implementation of the `PayloadProcessor` CRD
for the [wg-ai-gateway Payload Processing proposal](../../proposals/7-payload-processing.md).

## What It Does

The prototype demonstrates **body-based routing (BBR)** — reading a field from the JSON
request body and setting it as an HTTP header so that standard `HTTPRoute` header
matching can route to the correct backend.

```
Client                    Gateway                          Backends
  │                         │                                │
  │  POST /v1/chat/completions                               │
  │  body: {"model":"gpt-4"}                                 │
  │────────────────────────►│                                │
  │                         │                                │
  │                    PayloadProcessor (PreRouting)          │
  │                    Extracts model from body               │
  │                    Sets header: X-Gateway-Model-Name      │
  │                         │                                │
  │                    HTTPRoute matches header               │
  │                    X-Gateway-Model-Name: gpt-4           │
  │                         │───────────────────────────────►│ gpt4-backend
  │                         │                                │
```

## Two Processing Modes

### InProcess (CEL)

Uses CEL expressions evaluated directly in the gateway. No external service needed.

```yaml
apiVersion: ainetworking.x-k8s.io/v0alpha0
kind: PayloadProcessor
metadata:
  name: model-header-setter
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: ai-gateway
  phase: PreRouting
  processors:
  - name: extract-model
    type: InProcess
    failureMode: FailClosed
    inProcess:
      request:
        set:
        - name: X-Gateway-Model-Name
          value: 'json(request.body).model'
```

### ExtProc (External Processor)

Delegates processing to an external gRPC service using the
[Envoy ext_proc protocol](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/ext_proc/v3/external_processor.proto).

```yaml
apiVersion: ainetworking.x-k8s.io/v0alpha0
kind: PayloadProcessor
metadata:
  name: model-header-setter
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: ai-gateway
  phase: PreRouting
  processors:
  - name: extract-model
    type: ExtProc
    failureMode: FailClosed
    extProc:
      backendRef:
        name: ext-proc-bbr
        port: 4444
        kind: Service
```

## Directory Structure

| Directory | Purpose |
|-----------|---------|
| [api/](api/) | Go type definitions for the PayloadProcessor CRD |
| [controller/](controller/) | Minimal KRT-based controller (uses agentgateway framework) |
| [install-crd/](install-crd/) | CRD and RBAC YAML for installation |
| [ext-proc-server/](ext-proc-server/) | Reference ExtProc gRPC server implementation |
| [testdata/](testdata/) | Example Kubernetes resources for testing |
| [docs/](docs/) | Architecture and design documentation |

## CRD Schema

| Field | Description |
|-------|-------------|
| `spec.targetRef` | Gateway, ListenerSet, or HTTPRoute to attach to |
| `spec.phase` | `PreRouting` (before route selection) or `PostRouting` (after) |
| `spec.processors[]` | Ordered list of processing steps (max 16) |
| `spec.processors[].type` | `InProcess` (CEL) or `ExtProc` (external gRPC) |
| `spec.processors[].failureMode` | `FailClosed` (default) or `FailOpen` |
| `spec.processors[].inProcess` | CEL-based header mutations (set/add/remove) |
| `spec.processors[].extProc` | Backend reference to external processor service |

## Testing

### Prerequisites

- A Kubernetes cluster with a Gateway API implementation installed
- The Gateway API CRDs installed

### Install CRD

```bash
kubectl apply -f install-crd/
```

### Deploy Test Backends

```bash
# Deploy simulated LLM backends (gpt4, claude, default)
kubectl apply -f testdata/simulator-backends.yaml
```

### InProcess Mode

```bash
# Deploy Gateway, PayloadProcessor (InProcess), and HTTPRoutes
kubectl apply -f testdata/payload-processor-bbr.yaml
```

### ExtProc Mode

```bash
# Build and deploy the ext-proc server
docker build -t ext-proc-bbr:latest ext-proc-server/
# Load into your cluster (e.g., kind load docker-image ext-proc-bbr:latest)
kubectl apply -f ext-proc-server/deploy.yaml

# Deploy Gateway, PayloadProcessor (ExtProc), and HTTPRoutes
kubectl apply -f testdata/payload-processor-ext-proc.yaml
```

### Verify

```bash
# Should route to gpt4-backend
curl -X POST http://gateway:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}'

# Should route to claude-backend
curl -X POST http://gateway:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "claude", "messages": [{"role": "user", "content": "hello"}]}'

# Should route to default-backend
curl -X POST http://gateway:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "llama", "messages": [{"role": "user", "content": "hello"}]}'
```

## ExtProc Protocol Pattern

The ext-proc server implements a specific pattern for body-based routing that works
with streaming-capable gateways:

1. **Request headers arrive** (`end_of_stream=false`): Do not respond — defer until body is read
2. **Body chunks arrive**: Buffer silently (no per-chunk response needed)
3. **Final body chunk** (`end_of_stream=true`): Extract model, send two responses:
   - `RequestHeaders` response with header mutation (`X-Gateway-Model-Name`)
   - `RequestBody` response with `StreamedResponse` echoing the buffered body
4. **Response headers/body**: Pass through (echo body via `StreamedResponse`)

This pattern ensures the header mutation is applied before route selection while
preserving the original request body for the backend.

See [ext-proc-server/main.go](ext-proc-server/main.go) for the reference implementation.

## Controller

The [controller/](controller/) directory contains a standalone KRT-based controller
that watches `PayloadProcessor` CRDs and translates them into data plane configuration.

**Architecture:**
- Uses [KRT](https://pkg.go.dev/istio.io/istio/pkg/kube/krt) (Kubernetes Reconciliation Types)
  for reactive resource watching — the same framework used by agentgateway
- Core translation logic lives in [controller/pkg/translate.go](controller/pkg/translate.go),
  adapted from agentgateway's PayloadProcessor plugin
- Delivers policies to connected data plane instances via a minimal xDS delta server

**What it does:**
- Watches `PayloadProcessor`, `Gateway`, and `Service` resources via KRT collections
- For InProcess processors: emits transformation policies with CEL header expressions
- For ExtProc processors: emits ext-proc policies with resolved backend references
- Pushes agentgateway-compatible policies to the data plane via xDS (port 9978)

```bash
# Build the controller
cd controller && docker build -t payload-processor-controller .

# Or run locally (requires kubeconfig and PayloadProcessor CRD installed)
go run ./cmd/ --xds-port 9978
```

See [docs/architecture.md](docs/architecture.md) for details on the translation logic.

## Limitations

- **InProcess mode**: Only CEL expressions supported; no custom logic
- **ExtProc mode**: Requires the gateway to support the Envoy ext_proc protocol
- **FailOpen**: Accepted but behavior depends on gateway implementation
- **Timeout**: Field accepted but not enforced in this prototype
- **Status**: The CRD `.status` is not populated
- **Ordering**: Multiple processors become independent policies; ordering not guaranteed

## Reference Implementation

This prototype was developed and validated using [AgentGateway](https://github.com/agentgateway/agentgateway)
as the reference gateway implementation. The CRD design is gateway-agnostic and
can be implemented by any Gateway API provider that supports transformations or
the ext_proc protocol.
