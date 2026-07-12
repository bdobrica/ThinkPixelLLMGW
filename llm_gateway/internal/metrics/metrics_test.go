package metrics

import (
	"testing"
	"time"
)

func TestPrometheusMetrics_RecordRequest(t *testing.T) {
	// Create metrics instance
	m := NewPrometheusMetrics()

	// Test recording a request
	tags := map[string]string{
		"environment": "test",
		"team":        "engineering",
	}

	// This should not panic
	m.RecordRequest(
		"test-key-id",
		"test-key-name",
		tags,
		1000,                 // input tokens
		100,                  // cached tokens
		500,                  // output tokens
		0.05,                 // cost USD
		500*time.Millisecond, // latency
	)
	m.RecordStreamUsageMissing("openai", "gpt-4", "provider_missing")

	// Record another request with same key
	m.RecordRequest(
		"test-key-id",
		"test-key-name",
		tags,
		2000,          // input tokens
		200,           // cached tokens
		1000,          // output tokens
		0.10,          // cost USD
		1*time.Second, // latency
	)
	m.RecordStreamUsageMissing("openai", "gpt-4", "interrupted")
	m.RecordReadiness(true)
	m.RecordReadiness(true)
	m.RecordReadiness(false)

	// Record request with different key
	m.RecordRequest(
		"other-key-id",
		"other-key-name",
		map[string]string{},
		500,                  // input tokens
		0,                    // cached tokens
		250,                  // output tokens
		0.025,                // cost USD
		250*time.Millisecond, // latency
	)

	// Test that HTTPHandler is not nil
	handler := m.HTTPHandler()
	if handler == nil {
		t.Error("HTTPHandler() returned nil")
	}
}

func TestPrometheusMetricsReadinessCountsTransitionsOnly(t *testing.T) {
	m := NewPrometheusMetrics()
	m.RecordReadiness(true)
	m.RecordReadiness(true)
	m.RecordReadiness(false)
	families, err := m.registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]float64{}
	for _, family := range families {
		if family.GetName() != "llm_gateway_readiness_transitions_total" {
			continue
		}
		for _, metric := range family.Metric {
			counts[metric.Label[0].GetValue()] = metric.Counter.GetValue()
		}
	}
	if counts["ready"] != 1 || counts["unready"] != 1 {
		t.Fatalf("transition counts = %#v", counts)
	}
}

func TestNoopMetrics_RecordRequest(t *testing.T) {
	// Create noop metrics instance
	m := NewNoopMetrics()

	// This should not panic
	m.RecordRequest(
		"test-key-id",
		"test-key-name",
		map[string]string{},
		1000,
		100,
		500,
		0.05,
		500*time.Millisecond,
	)
	m.RecordReadiness(true)

	// Test that HTTPHandler is not nil
	handler := m.HTTPHandler()
	if handler == nil {
		t.Error("HTTPHandler() returned nil")
	}
}
