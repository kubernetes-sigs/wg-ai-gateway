# PayloadProcessor Architecture

## Overview

PayloadProcessor enables payload-aware processing at the gateway level. It runs
either **before** route selection (PreRouting) to influence routing decisions, or
**after** route selection (PostRouting) for guardrails and transformations.

## Control Plane Translation

A gateway controller watches `PayloadProcessor` CRDs and translates them into
data plane configuration. The translation depends on the processor type:

### InProcess → Transformation Policy

An InProcess processor with CEL expressions translates to a standard
transformation policy. For example:

```yaml
# PayloadProcessor CRD
processors:
- name: extract-model
  type: InProcess
  inProcess:
    request:
      set:
      - name: X-Gateway-Model-Name
        value: 'json(request.body).model'
```

Becomes an internal transformation policy that:
1. Buffers the request body (triggered by `request.body` reference in CEL)
2. Evaluates `json(request.body).model` against the JSON body
3. Sets the `X-Gateway-Model-Name` header with the result
4. Route selection then matches on this header

**Key insight**: No data plane changes needed — the controller emits standard
transformation policies that existing CEL engines handle.

### ExtProc → External Processor Policy

An ExtProc processor translates to an ext_proc filter configuration:

```yaml
# PayloadProcessor CRD
processors:
- name: extract-model
  type: ExtProc
  extProc:
    backendRef:
      name: ext-proc-bbr
      port: 4444
```

Becomes an ext_proc policy that:
1. Connects to the referenced Service via gRPC (h2c)
2. Sends request headers and body to the external processor
3. Applies header mutations from the processor's response
4. Forwards the (possibly modified) body to the backend

### Phase Mapping

| PayloadProcessor Phase | Internal Phase | When It Runs |
|------------------------|---------------|--------------|
| `PreRouting` | Gateway/Listener | Before `select_route()` — headers set here affect routing |
| `PostRouting` | Route | After route selected — for guardrails, enrichment |

## Data Plane Processing

### Request Flow (PreRouting ExtProc)

```
1. Client sends request
2. Gateway receives headers + body
3. Gateway-phase policies execute (including ext_proc)
   a. Headers sent to ext-proc server
   b. Body streamed to ext-proc server
   c. Ext-proc returns header mutations (e.g., X-Gateway-Model-Name: gpt-4)
   d. Mutations applied to request
4. Route selection runs — matches on mutated headers
5. Request forwarded to selected backend
```

### ExtProc Protocol Flow

The ext-proc server implements Envoy's `ExternalProcessor` gRPC service
with a specific pattern for body-based routing:

```
Gateway                          ExtProc Server
  │                                  │
  │  RequestHeaders (eos=false)      │
  │─────────────────────────────────►│
  │                                  │  (no response — wait for body)
  │  RequestBody chunk               │
  │─────────────────────────────────►│
  │                                  │  (buffer, no response)
  │  RequestBody (eos=true)          │
  │─────────────────────────────────►│
  │                                  │  Extract model from body
  │  RequestHeaders response         │
  │◄─────────────────────────────────│  {header_mutation: X-Gateway-Model-Name}
  │                                  │
  │  RequestBody response            │
  │◄─────────────────────────────────│  {body_mutation: StreamedResponse(body)}
  │                                  │
  │  [Route selection happens now]   │
  │                                  │
  │  ResponseHeaders                 │
  │─────────────────────────────────►│
  │  ResponseHeaders response        │
  │◄─────────────────────────────────│  (pass-through)
  │                                  │
  │  ResponseBody chunks             │
  │─────────────────────────────────►│
  │  ResponseBody response           │
  │◄─────────────────────────────────│  {body_mutation: StreamedResponse(echo)}
```

**Why RequestHeaders response for body mutations?**

The ext-proc sends header mutations via a `RequestHeaders` response (not
`RequestBody`) even though it's responding during body processing. This is
because the gateway applies `RequestHeaders` response mutations to the request
object synchronously — the mutations are visible to route selection. A
`RequestBody` response's header mutations would be applied asynchronously
(in the spawned body handler) and missed by routing.

**Why StreamedResponse for body echo?**

The gateway replaces the original body with a streaming body fed by ext-proc
responses. Without echoing the body back via `StreamedResponse`, the backend
receives an empty body. The ext-proc buffers the full body and sends it back
in the `RequestBody` response.

## CRD Design Decisions

See [implementation-notes.md](implementation-notes.md) for detailed design decisions including:
- API version choice (`v0alpha0`)
- Single `targetRef` vs list
- Hand-written deepcopy files
- Phase model (PreRouting/PostRouting)
