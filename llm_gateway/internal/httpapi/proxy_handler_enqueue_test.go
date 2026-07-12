package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/billing"
	"llm_gateway/internal/logging"
	"llm_gateway/internal/metrics"
	"llm_gateway/internal/models"
	"llm_gateway/internal/providers"
	"llm_gateway/internal/storage"
)

type failingSink struct{ err error }

func (s *failingSink) Enqueue(rec *logging.LogRecord) error { return s.err }
func (s *failingSink) Shutdown(ctx context.Context) error   { return nil }

type capturingSink struct{ records []*logging.LogRecord }

func (s *capturingSink) Enqueue(rec *logging.LogRecord) error {
	copyRecord := *rec
	s.records = append(s.records, &copyRecord)
	return nil
}

func (s *capturingSink) Shutdown(ctx context.Context) error { return nil }

type failingBillingEnqueuer struct{ err error }

func (e *failingBillingEnqueuer) Enqueue(ctx context.Context, update *billing.BillingUpdate) error {
	return e.err
}

type failingUsageEnqueuer struct{ err error }

func (e *failingUsageEnqueuer) Enqueue(ctx context.Context, record *models.UsageRecord) error {
	return e.err
}

type capturingUsageEnqueuer struct{ records []*models.UsageRecord }

func (e *capturingUsageEnqueuer) Enqueue(ctx context.Context, record *models.UsageRecord) error {
	copyRecord := *record
	e.records = append(e.records, &copyRecord)
	return nil
}

type capturingBillingEnqueuer struct{ updates []*billing.BillingUpdate }

func (e *capturingBillingEnqueuer) Enqueue(ctx context.Context, update *billing.BillingUpdate) error {
	copyUpdate := *update
	e.updates = append(e.updates, &copyUpdate)
	return nil
}

type capturingMetrics struct {
	requests     int
	inputTokens  int
	cachedTokens int
	outputTokens int
	costUSD      float64
	missing      int
}

func (m *capturingMetrics) HTTPHandler() http.Handler { return http.NotFoundHandler() }
func (m *capturingMetrics) RecordRequest(_ string, _ string, _ map[string]string, inputTokens, cachedTokens, outputTokens int, costUSD float64, _ time.Duration) {
	m.requests++
	m.inputTokens = inputTokens
	m.cachedTokens = cachedTokens
	m.outputTokens = outputTokens
	m.costUSD = costUSD
}
func (m *capturingMetrics) RecordStreamUsageMissing(_, _, _ string) { m.missing++ }

type stubProvider struct{}

func (p *stubProvider) ID() string   { return "provider-1" }
func (p *stubProvider) Name() string { return "stub" }
func (p *stubProvider) Type() string { return "openai" }
func (p *stubProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return nil, nil
}
func (p *stubProvider) ValidateCredentials(ctx context.Context) error { return nil }
func (p *stubProvider) Close() error                                  { return nil }

func TestHandleNonStreamingResponse_ReportsAsyncEnqueueFailures(t *testing.T) {
	requestID := uuid.New().String()
	apiKeyID := uuid.New()
	reportedTargets := make([]string, 0, 3)
	originalReporter := reportAsyncEnqueueFailure
	reportAsyncEnqueueFailure = func(target, requestID, apiKeyID string, err error, keyvals ...any) {
		reportedTargets = append(reportedTargets, target)
	}
	defer func() {
		reportAsyncEnqueueFailure = originalReporter
	}()

	deps := &Dependencies{
		Logger:        &failingSink{err: errors.New("log queue unavailable")},
		BillingWorker: &failingBillingEnqueuer{err: errors.New("billing queue unavailable")},
		UsageWorker:   &failingUsageEnqueuer{err: errors.New("usage queue unavailable")},
		Metrics:       metrics.NewNoopMetrics(),
	}

	recorder := httptest.NewRecorder()
	apiKeyRecord := &auth.APIKeyRecord{
		ID:   apiKeyID.String(),
		Name: "Test Key",
		Tags: map[string]string{"env": "test"},
	}
	response := &providers.ChatResponse{
		StatusCode:      http.StatusOK,
		Body:            []byte(`{"id":"resp-1"}`),
		CostUSD:         1.25,
		InputTokens:     120,
		OutputTokens:    45,
		CachedTokens:    5,
		ReasoningTokens: 2,
	}

	deps.handleNonStreamingResponse(
		recorder,
		response,
		apiKeyRecord,
		requestID,
		"gpt-4",
		"gpt-4",
		&stubProvider{},
		map[string]any{"model": "gpt-4"},
		time.Now().Add(-150*time.Millisecond),
		150*time.Millisecond,
		nil,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if recorder.Body.String() != `{"id":"resp-1"}` {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}

	wantTargets := []string{"request log", "billing update", "usage record"}
	if !reflect.DeepEqual(reportedTargets, wantTargets) {
		t.Fatalf("reported targets = %v, want %v", reportedTargets, wantTargets)
	}
}

type interruptedStream struct {
	data   []byte
	err    error
	served bool
}

func (s *interruptedStream) Read(p []byte) (int, error) {
	if !s.served {
		s.served = true
		return copy(p, s.data), nil
	}
	if s.err != nil {
		err := s.err
		s.err = nil
		return 0, err
	}
	return 0, io.EOF
}

func (s *interruptedStream) Close() error { return nil }

func TestHandleStreamingResponse_InterruptedStreamDoesNotEmitDoneAndRecordsError(t *testing.T) {
	requestID := uuid.New().String()
	apiKeyID := uuid.New()
	logSink := &capturingSink{}
	usageSink := &capturingUsageEnqueuer{}
	deps := &Dependencies{
		Logger:      logSink,
		UsageWorker: usageSink,
		Metrics:     metrics.NewNoopMetrics(),
	}

	stream := &interruptedStream{
		data: []byte("data: {\"usage\":{\"input_tokens\":100,\"output_tokens\":40}}\n\n"),
		err:  errors.New("provider stream reset"),
	}
	response := &providers.ChatResponse{
		StatusCode: http.StatusOK,
		Stream:     stream,
	}
	recorder := httptest.NewRecorder()
	apiKeyRecord := &auth.APIKeyRecord{ID: apiKeyID.String(), Name: "Test Key"}

	deps.handleStreamingResponse(
		recorder,
		httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil),
		response,
		apiKeyRecord,
		requestID,
		"gpt-4",
		"gpt-4",
		&stubProvider{},
		map[string]any{"model": "gpt-4", "stream": true},
		time.Now().Add(-200*time.Millisecond),
		100*time.Millisecond,
		nil,
	)

	if bytes.Contains(recorder.Body.Bytes(), []byte("data: [DONE]")) {
		t.Fatalf("unexpected DONE marker in interrupted stream response: %s", recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("input_tokens")) {
		t.Fatalf("expected streamed payload to be forwarded before interruption, got %s", recorder.Body.String())
	}
	if len(logSink.records) != 1 {
		t.Fatalf("expected one log record, got %d", len(logSink.records))
	}
	if logSink.records[0].Error == "" {
		t.Fatal("expected interrupted stream to be logged as an error")
	}
	responsePayload, ok := logSink.records[0].ResponsePayload.(map[string]any)
	if !ok {
		t.Fatalf("expected response payload map, got %T", logSink.records[0].ResponsePayload)
	}
	if completed, _ := responsePayload["completed"].(bool); completed {
		t.Fatal("expected completed=false for interrupted stream")
	}
	if len(usageSink.records) != 1 {
		t.Fatalf("expected one usage record, got %d", len(usageSink.records))
	}
	if usageSink.records[0].ErrorMessage == "" {
		t.Fatal("expected usage record to include the stream interruption error")
	}
}

func TestHandleStreamingResponse_AccountsTerminalUsageExactlyOnce(t *testing.T) {
	requestID := uuid.New().String()
	apiKeyID := uuid.New()
	logSink := &capturingSink{}
	usageSink := &capturingUsageEnqueuer{}
	billingSink := &capturingBillingEnqueuer{}
	metricSink := &capturingMetrics{}
	deps := &Dependencies{Logger: logSink, UsageWorker: usageSink, BillingWorker: billingSink, Metrics: metricSink}
	model := &models.Model{PricingComponents: []models.PricingComponent{
		{Direction: models.PricingDirectionInput, Modality: models.PricingModalityText, Unit: models.PricingUnit1KTokens, Price: 0.01},
		{Direction: models.PricingDirectionOutput, Modality: models.PricingModalityText, Unit: models.PricingUnit1KTokens, Price: 0.03},
		{Direction: models.PricingDirectionCache, Modality: models.PricingModalityText, Unit: models.PricingUnit1KTokens, Price: 0.005},
	}}
	stream := io.NopCloser(bytes.NewBufferString(
		"data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n" +
			"data: {\"usage\":{\"prompt_tokens\":120,\"completion_tokens\":45,\"prompt_tokens_details\":{\"cached_tokens\":20},\"completion_tokens_details\":{\"reasoning_tokens\":5}}}\n\n" +
			"data: [DONE]\n\n",
	))
	recorder := httptest.NewRecorder()

	deps.handleStreamingResponse(
		recorder,
		httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil),
		&providers.ChatResponse{StatusCode: http.StatusOK, Stream: stream},
		&auth.APIKeyRecord{ID: apiKeyID.String(), Name: "Test Key"},
		requestID, "gpt-4", "gpt-4", &stubProvider{},
		map[string]any{"model": "gpt-4", "stream": true}, time.Now(), 10*time.Millisecond,
		&storage.ModelWithDetails{Model: model, PricingComponents: model.PricingComponents},
	)

	if len(usageSink.records) != 1 || len(billingSink.updates) != 1 || len(logSink.records) != 1 {
		t.Fatalf("enqueue counts: usage=%d billing=%d logs=%d", len(usageSink.records), len(billingSink.updates), len(logSink.records))
	}
	record := usageSink.records[0]
	if record.InputTokens != 120 || record.OutputTokens != 45 || record.CachedTokens != 20 || record.ReasoningTokens != 5 {
		t.Fatalf("unexpected usage record: %#v", record)
	}
	if record.ErrorMessage != "" {
		t.Fatalf("unexpected accounting error: %s", record.ErrorMessage)
	}
	if metricSink.requests != 1 || metricSink.inputTokens != 120 || metricSink.outputTokens != 45 || metricSink.cachedTokens != 20 || metricSink.costUSD <= 0 || metricSink.missing != 0 {
		t.Fatalf("unexpected metrics: %#v", metricSink)
	}
	if billingSink.updates[0].CostUSD != metricSink.costUSD {
		t.Fatalf("billing cost %f differs from metric cost %f", billingSink.updates[0].CostUSD, metricSink.costUSD)
	}
}

func TestHandleStreamingResponse_CompletedWithoutUsageRecordsUnknownAccounting(t *testing.T) {
	usageSink := &capturingUsageEnqueuer{}
	metricSink := &capturingMetrics{}
	deps := &Dependencies{Logger: &capturingSink{}, UsageWorker: usageSink, Metrics: metricSink}
	stream := io.NopCloser(bytes.NewBufferString("data: {\"choices\":[]}\n\ndata: [DONE]\n\n"))

	deps.handleStreamingResponse(
		httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil),
		&providers.ChatResponse{StatusCode: http.StatusOK, Stream: stream},
		&auth.APIKeyRecord{ID: uuid.New().String(), Name: "Test Key"}, uuid.New().String(),
		"gpt-4", "gpt-4", &stubProvider{}, map[string]any{"stream": true}, time.Now(), time.Millisecond, nil,
	)

	if len(usageSink.records) != 1 || usageSink.records[0].ErrorMessage == "" {
		t.Fatalf("expected one unknown-accounting usage record, got %#v", usageSink.records)
	}
	if metricSink.missing != 1 {
		t.Fatalf("missing usage metric count = %d, want 1", metricSink.missing)
	}
}

func TestHandleStreamingResponse_ClientCancellationIsInterrupted(t *testing.T) {
	usageSink := &capturingUsageEnqueuer{}
	metricSink := &capturingMetrics{}
	deps := &Dependencies{Logger: &capturingSink{}, UsageWorker: usageSink, Metrics: metricSink}
	stream := &interruptedStream{err: context.Canceled}

	deps.handleStreamingResponse(
		httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil),
		&providers.ChatResponse{StatusCode: http.StatusOK, Stream: stream},
		&auth.APIKeyRecord{ID: uuid.New().String(), Name: "Test Key"}, uuid.New().String(),
		"gpt-4", "gpt-4", &stubProvider{}, map[string]any{"stream": true}, time.Now(), time.Millisecond, nil,
	)

	if len(usageSink.records) != 1 || !strings.Contains(usageSink.records[0].ErrorMessage, "context canceled") {
		t.Fatalf("expected cancellation error record, got %#v", usageSink.records)
	}
	if metricSink.missing != 1 {
		t.Fatalf("missing usage metric count = %d, want 1", metricSink.missing)
	}
}
