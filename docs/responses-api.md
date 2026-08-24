# Responses API compatibility

**Contract snapshot:** August 24, 2026
**Authoritative reference:** [OpenAI create response API](https://developers.openai.com/api/reference/resources/responses/methods/create)

The gateway is implementing `/v1/responses` as a separate, item-oriented protocol. The typed contract and validation foundation exists, but the route is not registered yet. Until Phase 7 Step 22 is complete, clients receive the normal not-found response and must continue using `/v1/chat/completions`.

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

“Native” and “translated” describe the selected implementation path, not current endpoint availability. The `responses_enabled` capability remains false for every provider until the corresponding handler, orchestration, and conformance suite are complete.

| Capability | OpenAI | Vertex AI | AWS Bedrock |
|---|---|---|---|
| Responses transport | Native pass-through planned | Gateway translation planned | Gateway translation planned |
| Gateway state storage | Planned | Planned | Planned |
| Reasoning items | Native planned | Unavailable until a lossless mapping exists | Unavailable until a lossless mapping exists |
| Function tools | Native planned | Translation planned | Unavailable until native tool translation exists |
| Parallel function calls | Native planned | Translation planned | Unavailable |
| Responses SSE events | Native event path planned | Gateway translation planned | Gateway translation planned |
| Web search | Disabled pending hosted-tool framework | Disabled | Disabled |
| File search | Disabled pending hosted-tool framework | Disabled | Disabled |
| Code interpreter | Disabled pending hardened sandbox | Disabled | Disabled |

Model capability resolution intersects these provider capabilities with existing catalog flags such as reasoning, function calling, parallel function calling, streaming, and web search. A catalog flag can narrow support but cannot enable a provider feature marked unavailable.

## Explicitly deferred

This snapshot does not register any Responses resource route, persist response state, call providers, execute tools, or emit Responses SSE events. Conversations, prompts, MCP, computer use, image generation, remote shell, and newer item/tool variants are outside the first supported subset. They must be added with typed schemas, validation, capability gates, fixtures, and documentation before being accepted.

Custom function tools are client-executed: the gateway will return a `function_call`, and the client will submit the matching `function_call_output`. Web search, file search, and code interpreter are server-managed and remain disabled until their security and ownership requirements are implemented.

## Testing

Run the contract suite with:

```bash
cd llm_gateway
go test ./internal/responses
```

The later endpoint suite must additionally verify official OpenAI SDK behavior. Passing the Step 20 tests establishes schema stability only; it does not claim runtime `/v1/responses` compatibility.
