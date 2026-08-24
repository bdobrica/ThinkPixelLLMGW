# Responses API compatibility

**Contract snapshot:** August 24, 2026
**Authoritative reference:** [OpenAI create response API](https://developers.openai.com/api/reference/resources/responses/methods/create)

The gateway implements `/v1/responses` as a separate, item-oriented protocol. `POST /v1/responses` currently accepts non-streaming requests for models routed to a native OpenAI provider. The route uses the same API-key authentication, model authorization, rate limit, budget, request-body limit, usage, billing, metrics, and privacy boundaries as Chat Completions.

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
| Responses SSE events | Deferred to Step 25 | Gateway translation planned | Gateway translation planned |
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

Streaming and background creation are explicitly rejected until the Responses SSE state machine and durable background execution are installed. `GET`, `DELETE`, and cancellation resource operations, translated providers, conversations, prompts, MCP, computer use, image generation, remote shell, and newer item/tool variants remain deferred. They must be added with typed schemas, validation, capability gates, fixtures, and documentation before being accepted.

Custom function tools are client-executed: the gateway will return a `function_call`, and the client will submit the matching `function_call_output`. Web search, file search, and code interpreter are server-managed and remain disabled until their security and ownership requirements are implemented.

## Testing

Run the contract suite with:

```bash
cd llm_gateway
go test ./internal/responses
```

The endpoint suite must additionally verify official OpenAI SDK behavior before the Responses API is advertised as generally compatible.
