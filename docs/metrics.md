# Metrics Implementation

The LLM Gateway now exposes Prometheus metrics at the `/metrics` endpoint.

## Available Metrics

Request, token, cost, and latency metrics include the following labels:
- `api_key_id`: The UUID of the API key making the request
- `api_key_name`: The human-readable name of the API key

### Counter Metrics

1. **`llm_gateway_requests_total`**
   - Total number of LLM API requests processed
   - Increments by 1 for each request

2. **`llm_gateway_input_tokens_total`**
   - Total number of input tokens processed
   - Increments by the number of input tokens in each request

3. **`llm_gateway_cached_tokens_total`**
   - Total number of cached input tokens (prompt cache hits)
   - Increments by the number of cached tokens in each request

4. **`llm_gateway_output_tokens_total`**
   - Total number of output tokens generated
   - Increments by the number of output tokens in each response

5. **`llm_gateway_cost_usd_total`**
   - Total cost tracked in USD
   - Increments by the calculated cost for each request

6. **`llm_gateway_stream_usage_missing_total`**
   - Counts streams for which terminal provider usage was unavailable
   - Labels: `provider`, `model`, and `reason` (`provider_missing` or `interrupted`)
   - Alert on any sustained increase and reconcile affected requests against provider billing exports

7. **`llm_gateway_readiness_transitions_total`**
   - Counts actual readiness changes, including the initially observed state
   - Uses only the bounded `state` label (`ready` or `unready`); dependency errors are not labels

### Gauge Metrics

8. **`llm_gateway_ready`**
   - Current `/ready` result: `1` for ready and `0` for unavailable

### Histogram Metrics

9. **`llm_gateway_request_duration_seconds`**
   - Request latency distribution in seconds
   - Buckets: 0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0 seconds
   - Provides `_sum`, `_count`, and `_bucket` metrics for percentile calculations

## Usage Examples

### Query total requests per API key
```promql
llm_gateway_requests_total
```

### Query total cost per API key
```promql
llm_gateway_cost_usd_total
```

### Query request rate (requests per second)
```promql
rate(llm_gateway_requests_total[5m])
```

### Query 95th percentile latency
```promql
histogram_quantile(0.95, rate(llm_gateway_request_duration_seconds_bucket[5m]))
```

### Query average tokens per request
```promql
rate(llm_gateway_input_tokens_total[5m]) / rate(llm_gateway_requests_total[5m])
```

### Query cost rate (USD per minute)
```promql
rate(llm_gateway_cost_usd_total[1m]) * 60
```

## API Key Tags

While API key tags are passed to the metrics recording function, they are not directly exposed as Prometheus labels due to cardinality concerns. Prometheus requires consistent label names across all metric instances, and dynamic tags would create unpredictable cardinality.

To correlate metrics with API key tags:
1. Use the `api_key_id` label to identify the API key in metrics
2. Query the API key metadata via the admin API to retrieve associated tags
3. Join this data in your monitoring dashboard or alerting system

## Implementation Notes

- Metrics are recorded for both streaming and non-streaming requests
- OpenAI streaming requests ask for a terminal usage chunk. Complete SSE events are parsed without buffering the generated response, and reported input, output, cached, and reasoning usage drives the same database-configured pricing calculation as non-streaming requests.
- An interrupted stream or a completed stream without terminal usage produces an unknown-accounting usage record and increments `llm_gateway_stream_usage_missing_total`; it is not treated as a successful zero-cost request. Automatic estimation/reconciliation is not yet implemented.
- Cost calculations use the accurate pricing components from the model configuration
- Latency includes the full gateway processing time, not just provider response time
- The `/metrics` endpoint is publicly accessible (no authentication required)

## Prometheus Configuration

Add the following to your Prometheus `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'llm_gateway'
    scrape_interval: 15s
    static_configs:
      - targets: ['gateway:8080']  # Adjust host:port as needed
```

## Grafana Dashboard

You can create a Grafana dashboard with panels like:

- **Request Rate**: `rate(llm_gateway_requests_total[5m])`
- **Token Usage**: Stacked graph with input/output/cached tokens
- **Cost Over Time**: `increase(llm_gateway_cost_usd_total[1h])`
- **Latency Percentiles**: P50, P95, P99 from the histogram
- **Top API Keys by Cost**: `topk(10, llm_gateway_cost_usd_total)`

## Future Enhancements

Potential improvements for the metrics system:
- Add model name as a label (requires cardinality management)
- Add provider name as a label
- Add endpoint path as a label (e.g., `/v1/chat/completions`, `/v1/embeddings`)
- Add error rate metrics with error type labels
- Add streaming vs non-streaming request distinction
- Add automatic provider-billing reconciliation for unknown streaming usage
