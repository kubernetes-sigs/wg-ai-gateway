# PayloadProcessor POC — Implementation Notes

Companion to [plan-payloadProcessorPoc.prompt.md](plan-payloadProcessorPoc.prompt.md).
This file tracks design decisions, deviations from the plan, and open questions as we implement.

---

## Step 1: CRD Type Definitions

### Status: ✅ Complete

### Files Created
- `controller/api/v0alpha0/ainetworking/doc.go` ✅
- `controller/api/v0alpha0/ainetworking/payload_processor_types.go` ✅
- `controller/api/v0alpha0/ainetworking/zz_generated.register.go` ✅ (hand-written, matches register-gen pattern)
- `controller/api/v0alpha0/ainetworking/zz_generated.deepcopy.go` ✅ (hand-written, matches controller-gen pattern)

### Deviations from Plan
| Plan Said | Actual | Reason |
|-----------|--------|--------|
| API version `v1alpha1` | `v0alpha0` | Per review — this is a prototype, not alpha-ready |
| Directory `controller/api/v1alpha1/ainetworking/` | `controller/api/v0alpha0/ainetworking/` | Matches version change |
| Run `make generate` for deepcopy/register | Hand-written files | `hack/generate.sh` is hardcoded to `v1alpha1/agentgateway`; writing them by hand was simpler for the POC |

### Design Decisions Made

**Single `targetRef` vs list `targetRefs`:**
- Chose **single `targetRef`** for simplicity. Gateway API's policy attachment (GEP-713) supports both patterns. The Payload Processing proposal also uses single target. Can extend to a list later if needed.

**`HeaderName` pattern — no pseudo-headers:**
- AgentgatewayPolicy's `HeaderName` allows HTTP/2 pseudo-headers (`:authority`, `:method`, `:path`, `:scheme`, `:status`) via a special validation rule.
- Our `HeaderName` uses a simpler regex that only allows regular headers (e.g. `X-Gateway-Model-Name`).
- Not needed for the POC use case (body-based routing sets regular headers). Can add pseudo-header support later.

**No `body` or `metadata` fields in `InProcessTransform`:**
- AgentgatewayPolicy's `Transform` type includes `body` (CEL expression to rewrite the body) and `metadata` (CEL-evaluated values stored for later policy use).
- Our `InProcessTransform` only has `set`, `add`, `remove` — header mutation only.
- This is intentional for the POC scope. These fields can be added when body transformation or metadata use cases are needed.

### Build Validation
```
$ go build ./controller/api/v0alpha0/...   # ✅ exit code 0
$ go build ./controller/...                # ✅ exit code 0, no regressions
```

---

## Step 2: Controller Registration & Collection

### Status: ✅ Complete

### Files Modified
- `controller/pkg/agentgateway/plugins/collection.go` — added `ainetworking` import, `PayloadProcessors` collection field, informer initialization (~6 lines)

### Design Decisions Made

**No changes to `setup.go`:**
- The plan said to modify `setup.go`, but the informer is created inside `NewAgwCollections()` in `collection.go`, which `setup.go` already calls. No `setup.go` modification needed.

**No changes to `policySelector`:**
- `policyselection.NewSelector` is specific to `AgentgatewayPolicy` and `BackendTLSPolicy`. PayloadProcessor policies flow through the plugin system instead, so the selector doesn't need modification.

### Build Validation
```
$ go build ./controller/...   # ✅ exit code 0, no regressions
```

---

## Step 3: Controller Translation Plugin

### Status: ✅ Complete

### Files Created
- `controller/pkg/agentgateway/plugins/payload_processor_plugin.go` (~190 lines)

### Files Modified
- `controller/pkg/controller/start.go` — 1 line added: registered `NewPayloadProcessorPlugin(agw)` in `Plugins()`

### Design Decisions Made

**Reuses `convertTransformSpec()` from `traffic_plugin.go`:**
- Instead of duplicating CEL validation logic, the plugin converts `InProcessTransform` → agentgateway `Transform` type → calls `convertTransformSpec()`.
- This means any improvements to CEL validation in `traffic_plugin.go` automatically benefit PayloadProcessor.

**PayloadProcessor GVK defined locally in the plugin:**
- Rather than adding to `wellknown/agw.go` (which is agentgateway-specific), the `PayloadProcessorGVK` is defined at the top of the plugin file.
- Can be moved to a `wellknown/ainetworking.go` in the future.

**Emits standard `TrafficPolicySpec_Transformation` policies:**
- The plugin outputs the same policy format as AgentgatewayPolicy transformations. This means the existing Rust data plane receives and processes them identically — no Rust changes needed.
- Phase mapping: `PreRouting` → `GATEWAY`, `PostRouting` → `ROUTE` (same as AgentgatewayPolicy).

**No status reporting in POC:**
- `NewPayloadProcessorPlugin` returns `nil` for the status collection. Status conditions (Accepted/Attached) are not implemented in the POC.

**Target resolution uses existing `utils` helpers:**
- `buildPolicyTarget()` delegates to `utils.GatewayTarget()`, `utils.RouteTarget()`, `utils.ListenerSetTarget()` rather than constructing proto types directly.

### Build Validation
```
$ go build ./controller/...   # ✅ exit code 0, no regressions
```

---

## Steps 4 & 5: Rust Data Plane — No Changes Needed

### Status: ✅ Complete (no modifications required)

### Key Realization
The Go translation plugin (Step 3) emits PayloadProcessor configs as **standard `TrafficPolicySpec_Transformation` policies** with the correct phase. The existing Rust data plane processes these identically to `AgentgatewayPolicy` transformations through the existing pipeline:

1. Go plugin emits `TrafficPolicySpec_Transformation` with `phase: GATEWAY`
2. Rust receives via xDS → `transformation_from_proto()` → `TrafficPolicy::Transformation`
3. Stored in `GatewayPolicies.transformation` (for GATEWAY phase) or `RoutePolicies.transformation` (for ROUTE phase)
4. Executed in `apply_gateway_policies()` (pre-routing) or `apply_request_policies()` (post-routing)
5. CEL expressions referencing `request.body` automatically trigger body buffering via `ContextBuilder`

### Deviations from Plan
| Plan Said | Actual | Reason |
|-----------|--------|--------|
| Create `crates/agentgateway/src/http/payload_processor.rs` (~120 lines) | No file created | Plugin emits standard transformation policies; existing Rust code handles them |
| Add `pub mod payload_processor;` to `http/mod.rs` | No change | Same reason |
| Add `payload_processors` field to `GatewayPolicies` and `RoutePolicies` | No change | Reuses existing `transformation` field |
| Modify `httpproxy.rs` to execute processors | No change | Existing `apply_gateway_policies()` and `apply_request_policies()` already execute transformations |

### What This Means for Future Work
- **`FailOpen` failure mode is NOT handled.** The existing `Transformation::apply_request()` simply applies the CEL mutation. If CEL evaluation fails, it silently does nothing (the CEL executor logs a trace-level message but doesn't error). This is effectively fail-open for individual header expressions but fail-closed if the overall policy can't be parsed. True per-processor fail-open/fail-closed will require a new Rust type wrapping `Transformation`.
- **Processor ordering across multiple processors is NOT preserved.** Since each processor becomes an independent transformation policy, they merge via the existing precedence rules rather than executing in declared order within the CRD. For the POC (usually one processor per CRD), this is fine.
- **Timeout per-processor is NOT enforced.** The timeout field in the CRD is accepted but not passed through to the data plane.

### Build Validation
```
$ cargo build -p agentgateway   # ✅ Finished in 2m03s, no changes needed
```

---

## Next Steps
- [x] Step 6: E2E Validation — create test manifests and documentation ✅

---

## Step 6: E2E Validation & Documentation

### Status: ✅ Complete

### Files Created
- `Jackie/testdata/payload-processor-bbr.yaml` — full K8s example with Gateway, PayloadProcessor, HTTPRoutes, and backend Services
- `Jackie/README.md` — POC documentation with architecture, usage, limitations, and comparison to AgentgatewayPolicy

### Test Manifest Design
The test YAML includes:
- 1 Gateway (`ai-gateway`)
- 1 PayloadProcessor targeting the Gateway with `PreRouting` phase and `json(request.body).model` → `X-Gateway-Model-Name`
- 3 HTTPRoutes: `gpt4-route` (matches `gpt-4`), `claude-route` (matches `claude`), `fallback-route` (catch-all)
- 3 backend Services (placeholder — need real pods for actual cluster testing)

### What Needs a Real Cluster to Verify
- [ ] `kubectl apply` succeeds (CRD must be registered first)
- [ ] Controller logs show PayloadProcessor translated to transformation policy
- [ ] Requests with `{"model":"gpt-4"}` body get routed to gpt4-backend
- [ ] Requests with `{"model":"claude"}` body get routed to claude-backend
- [ ] Requests with unknown models fall through to default-backend
- [ ] Requests without a body (e.g., GET /health) are unaffected

---

## Summary of All Changes

### New Files Created: 8
| File | Lines | Purpose |
|------|-------|---------|
| `controller/api/v0alpha0/ainetworking/doc.go` | 5 | Package + kubebuilder markers |
| `controller/api/v0alpha0/ainetworking/payload_processor_types.go` | ~200 | CRD type definitions |
| `controller/api/v0alpha0/ainetworking/zz_generated.register.go` | ~45 | Scheme registration |
| `controller/api/v0alpha0/ainetworking/zz_generated.deepcopy.go` | ~165 | DeepCopy implementations |
| `controller/pkg/agentgateway/plugins/payload_processor_plugin.go` | ~190 | Translation plugin |
| `Jackie/testdata/payload-processor-bbr.yaml` | ~130 | E2E test manifests |
| `Jackie/README.md` | ~140 | POC documentation |
| `Jackie/implementation-notes.md` | ~160 | This file |

### Existing Files Modified: 2
| File | Lines Changed | What |
|------|---------------|------|
| `controller/pkg/agentgateway/plugins/collection.go` | ~6 | Added import, collection field, informer |
| `controller/pkg/controller/start.go` | 1 | Registered plugin in `Plugins()` |

### Existing Files NOT Modified (per plan deviation): 4
| File | Plan Said | Why Not |
|------|-----------|---------|
| `controller/pkg/setup/setup.go` | Modify | Informer created in collection.go instead |
| `crates/agentgateway/src/http/payload_processor.rs` | Create | Plugin emits standard transformation policies |
| `crates/agentgateway/src/store/binds.rs` | Modify | Reuses existing `transformation` field |
| `crates/agentgateway/src/proxy/httpproxy.rs` | Modify | Existing transformation execution handles it |

### Build Results
- `go build ./controller/...` — ✅ pass
- `cargo build -p agentgateway` — ✅ pass (no changes needed)
