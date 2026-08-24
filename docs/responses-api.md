# Responses API compatibility

**Contract snapshot:** August 24, 2026
**Authoritative reference:** [OpenAI create response API](https://developers.openai.com/api/reference/resources/responses/methods/create)

The gateway implements `/v1/responses` as a separate, item-oriented protocol. `POST /v1/responses` currently accepts non-streaming requests for models routed to a native OpenAI provider. Stored responses can be retrieved with `GET /v1/responses/{response_id}` and soft-deleted with `DELETE /v1/responses/{response_id}`. These routes use the same API-key authentication and tenant boundary as creation.

The snapshot is intentionally explicit. A future OpenAI field or item type is not automatically supported merely because the gateway can decode JSON. Unsupported top-level fields, input variants, tools, includes, and incompatible parameter combinations fail validation before a provider request can become billable.

## Step 20 contract

The Go contract under `llm_gateway/internal/responses` currently represents:

- string input or an ordered input-item array;
- message items with text, image, or file content parts;
- reasoning, function-call, and function-call-output items;
- ordered message, reasoning, function, web-search, file-search, and code-interpreter output data;
- `queued`, `in_progress`, `completed`, `incomplete`, `failed`, and `cancelled` response states;
- input, cached-input, output, reasoning, and total token usage;
- state, streaming, reasoning, text format, tool, include, sampling, metadata, and truncation request controls;
- OpenAI-shaped invalid-request error envelopes.

Golden fixtures pin heterogeneous output ordering, `null` versus present lifecycle fields, response and item identifiers, cached/reasoning token details, string-or-array input, functions, and continuation controls.

The contract requires `model` and `input` for this gateway. `truncation` accepts `auto` or `disabled`; omitted truncation is interpreted by later orchestration as `disabled`. `previous_response_id` and `conversation` are mutually exclusive. Background streaming is deliberately rejected in this snapshot until background execution and event replay are implemented.

## Capability matrix

“Native” and “translated” describe the selected implementation path. Native OpenAI Responses transport is runtime-enabled; every translated route remains disabled until its item mapping and conformance suite are complete.

| Capability | OpenAI | Vertex AI | AWS Bedrock |
|---|---|---|---|
| Responses transport | Native non-streaming enabled | Gateway translation planned | Gateway translation planned |
| Continuation state | Provider-managed, gateway ID mapping | Gateway-managed planned | Gateway-managed planned |
| Reasoning items | Native enabled when the model catalog permits | Unavailable until a lossless mapping exists | Unavailable until a lossless mapping exists |
| Function tools | Native enabled when the model catalog permits | Translation planned | Unavailable until native tool translation exists |
| Parallel function calls | Native enabled when the model catalog permits | Translation planned | Unavailable |
| Responses SSE events | Typed encoder/state machine implemented; route wiring pending | Gateway translation planned | Gateway translation planned |
| Web search | Disabled pending hosted-tool framework | Disabled | Disabled |
| File search | Disabled pending hosted-tool framework | Disabled | Disabled |
| Code interpreter | Disabled pending hardened sandbox | Disabled | Disabled |

Model capability resolution intersects these provider capabilities with existing catalog flags such as reasoning, function calling, parallel function calling, streaming, and web search. A catalog flag can narrow support but cannot enable a provider feature marked unavailable.

## Stored state and retention

Step 21 adds PostgreSQL records for response envelopes, ordered input/output items, predecessor links, tool execution records, terminal usage/errors, and provider correlation IDs. Public response, item, call, and tool-execution IDs use independent cryptographically random namespaces; they are not derived from internal request UUIDs.

Every lookup includes the authenticated API-key owner. Missing, foreign-owned, expired, deleted, and `store: false` records all produce the same not-found result. Only completed or incomplete stored responses may be predecessors. Queued, in-progress, failed, and cancelled responses are rejected as continuation roots without disclosing whether a foreign ID exists. New instructions remain properties of the new response; persistence does not copy predecessor instructions.

`store: true` records default to 30 days and `store: false` orchestration records default to one hour. Operators can change these durations and bounded-chain limits with the `RESPONSES_*` settings documented in `docs/env-variables.md`. Soft deletion makes a response immediately unavailable; the bounded cleanup operation physically removes expired rows and cascades to items/tools. Encrypted provider reasoning state is stored only in the opaque ciphertext column using the gateway encryption key, never in audit-visible JSON.

Terminalization locks the response, inserts final output items, and writes terminal status/usage/error in one transaction. Repeating the same terminal state is idempotent; a conflicting terminal transition fails. Startup orchestration can reconcile stale `in_progress` rows to a failed terminal state, while cleanup and reconciliation use bounded `SKIP LOCKED` batches for multi-instance safety.

For native OpenAI routes, the provider owns conversational model state. The gateway generates a separate public `resp_` ID and stores a tenant-scoped mapping to the upstream response ID. On continuation, the client supplies the gateway ID and only the mapped upstream ID is forwarded. This preserves authorization, deletion/retention, audit, and provider-routing boundaries without replaying history unnecessarily. Provider failover is not promised for this mode.

For future translated routes, the gateway context assembler replays ordered stored items. It never copies predecessor instructions: only the new request's instructions are applied. `truncation: "disabled"` fails closed when the model context limit would be exceeded. `"auto"` removes the oldest eligible items deterministically, treats function calls and outputs sharing a `call_id` as indivisible, and always preserves current instructions and input. If an exact model tokenizer is unavailable, the fallback estimate reserves ten percent of the context window.

## Explicitly deferred

Streaming and background creation are explicitly rejected until provider-stream translation and the HTTP streaming path (and, for background work, durable execution) are installed. Cancellation, retrieval query options/event replay, translated providers, conversations, prompts, MCP, computer use, image generation, remote shell, and newer item/tool variants remain deferred. They must be added with typed schemas, validation, capability gates, fixtures, and documentation before being accepted.

## Resource operations foundation

Step 26 begins with authenticated retrieval and deletion. Both operations scope every database query to the API-key owner and deliberately return the same `not_found` error for absent, foreign, expired, deleted, and non-stored IDs. Retrieval reconstructs the response envelope and ordered output items from durable state without exposing provider correlation IDs or persisted input items. Opaque reasoning content is decrypted only when `reasoning.encrypted_content` was requested in the stored response contract.

Deletion is a soft delete and returns the OpenAI-shaped `response.deleted` resource. Repeated deletion returns `not_found`; retention cleanup later physically removes the row and its dependent items/tool executions. Retrieval `include`/event-replay query parameters, cancellation, background workers, operational metrics, accounting reconciliation, capacity alerts, and official SDK examples remain pending.

Custom function tools are client-executed: the gateway returns an ordered `function_call`, and the client submits the matching `function_call_output`, normally in a new request linked with the gateway `previous_response_id`. The gateway does not execute that function. It validates function names, bounded object JSON Schemas, strict-schema requirements, named/required/auto/none/allowed tool choices, and JSON arguments before provider work. Submitted outputs must match an unresolved `call_id` in the same tenant-owned predecessor chain; unknown and duplicate outputs are rejected. Native parallel calls preserve provider order and receive gateway-owned item IDs.

Streamed function arguments and gateway orchestration round/call/time/token limits remain part of Step 23. Web search, file search, and code interpreter are server-managed and remain disabled until their security and ownership requirements are implemented.

## Hosted-tool framework

Step 24 has begun with a provider-neutral executor boundary. Installed executors expose a typed descriptor, argument validation, cancellable execution, progress updates, JSON results, and separate byte/call/nano-USD usage. A per-response runner applies bounded input/output sizes, call count, concurrency, and wall time. Call IDs are idempotent within a response: an identical retry reuses the first result, while reuse with different input is rejected.

Registration is not authorization. Hosted tools remain default-disabled and a call must be explicitly allowed at all three scopes: deployment, selected model, and authenticated API key. Executor health is checked before execution. Lifecycle events contain response/call/tool identifiers, safe status messages, and final usage; they do not contain tool arguments, outputs, credentials, or raw backend errors.

No production executor is registered or wired into response orchestration yet, so the capability matrix still correctly reports web search, file search, and code interpreter as unavailable. The web-search backend and egress policy, tenant-owned file/vector lifecycle, hardened code sandbox, durable execution records, metrics/circuit breakers, and model-resumption loop remain required before any hosted tool can be enabled.

## Streaming foundation

Step 25 begins with a gateway-owned SSE encoder/state machine. It emits one named `event:` line and one JSON `data:` line per frame, assigns every event a monotonically increasing `sequence_number`, and flushes after each complete frame. It rejects deltas before their item/content add events, mismatched indices, duplicate items, events after item or response completion, and usage on non-terminal response snapshots. JSON framing preserves valid UTF-8 and escapes embedded newlines without leaking provider-specific frames or Chat Completions `[DONE]` markers.

This foundation is not yet connected to `POST /v1/responses`; `stream: true` continues to fail explicitly. Native provider frame parsing and ID rewriting, translated-provider event production, request-cancellation propagation, terminal persistence/billing, hosted-tool events, journal replay, and HTTP/SDK compatibility tests remain required before streaming is enabled.

## Testing

Run the contract suite with:

```bash
cd llm_gateway
go test ./internal/responses
```

The endpoint suite must additionally verify official OpenAI SDK behavior before the Responses API is advertised as generally compatible.
