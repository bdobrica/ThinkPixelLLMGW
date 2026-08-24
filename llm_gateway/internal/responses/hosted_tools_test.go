package responses

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeHostedExecutor struct {
	mu      sync.Mutex
	calls   int
	healthy bool
	block   <-chan struct{}
	result  HostedToolResult
}

func (f *fakeHostedExecutor) Descriptor(context.Context) HostedToolDescriptor {
	return HostedToolDescriptor{Description: "fake", Healthy: f.healthy}
}
func (f *fakeHostedExecutor) Validate(input json.RawMessage) error {
	if string(input) == `{"reject":true}` {
		return errors.New("rejected")
	}
	return nil
}
func (f *fakeHostedExecutor) Execute(ctx context.Context, _ HostedToolRequest, emit func(HostedToolEvent)) (HostedToolResult, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	emit(HostedToolEvent{Message: "working"})
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return HostedToolResult{}, ctx.Err()
		}
	}
	return f.result, nil
}

func allowPolicy(owner uuid.UUID, model, toolType string) HostedToolPolicy {
	return HostedToolPolicy{
		DeploymentAllow: map[string]bool{toolType: true},
		ModelAllow:      map[string]map[string]bool{model: {toolType: true}},
		APIKeyAllow:     map[uuid.UUID]map[string]bool{owner: {toolType: true}},
	}
}

func TestHostedToolsDefaultDenyAndRequireEveryScope(t *testing.T) {
	owner := uuid.New()
	executor := &fakeHostedExecutor{healthy: true, result: HostedToolResult{Output: json.RawMessage(`{"ok":true}`)}}
	registry := NewHostedToolRegistry()
	if err := registry.Register("web_search", executor); err != nil {
		t.Fatal(err)
	}
	for name, policy := range map[string]HostedToolPolicy{
		"empty":      {},
		"deployment": {DeploymentAllow: map[string]bool{"web_search": true}},
		"model": {DeploymentAllow: map[string]bool{"web_search": true},
			ModelAllow: map[string]map[string]bool{"gpt-test": {"web_search": true}}},
	} {
		t.Run(name, func(t *testing.T) {
			runner, err := NewHostedToolRunner(registry, policy, owner, "resp_test", "gpt-test", HostedToolLimits{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = runner.Execute(context.Background(), "call_1", Tool{Type: "web_search"}, json.RawMessage(`{"query":"safe"}`), nil)
			if !errors.Is(err, ErrHostedToolDisabled) {
				t.Fatalf("expected default deny, got %v", err)
			}
		})
	}
	if executor.calls != 0 {
		t.Fatalf("disabled executor ran %d times", executor.calls)
	}
}

func TestHostedToolRunnerEmitsSafeEventsUsageAndIsIdempotent(t *testing.T) {
	owner := uuid.New()
	executor := &fakeHostedExecutor{healthy: true, result: HostedToolResult{Output: json.RawMessage(`{"results":[]}`), Usage: HostedToolUsage{CostNanoUSD: 1200}}}
	registry := NewHostedToolRegistry()
	if err := registry.Register("web_search", executor); err != nil {
		t.Fatal(err)
	}
	runner, err := NewHostedToolRunner(registry, allowPolicy(owner, "gpt-test", "web_search"), owner, "resp_test", "gpt-test", HostedToolLimits{})
	if err != nil {
		t.Fatal(err)
	}
	var events []HostedToolEvent
	result, err := runner.Execute(context.Background(), "call_1", Tool{Type: "web_search_preview"}, json.RawMessage(`{"query":"weather"}`), func(event HostedToolEvent) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage.Calls != 1 || result.Usage.InputBytes == 0 || result.Usage.OutputBytes == 0 || result.Usage.CostNanoUSD != 1200 {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}
	if len(events) != 3 || events[0].Type != "tool.execution.started" || events[1].Type != "tool.execution.progress" || events[2].Type != "tool.execution.completed" {
		t.Fatalf("unexpected events: %#v", events)
	}
	if _, err := runner.Execute(context.Background(), "call_1", Tool{Type: "web_search"}, json.RawMessage(`{"query":"weather"}`), nil); err != nil {
		t.Fatal(err)
	}
	if executor.calls != 1 {
		t.Fatalf("idempotent retry executed %d times", executor.calls)
	}
	if _, err := runner.Execute(context.Background(), "call_1", Tool{Type: "web_search"}, json.RawMessage(`{"query":"other"}`), nil); !errors.Is(err, ErrHostedToolCallReuse) {
		t.Fatalf("expected call reuse rejection, got %v", err)
	}
}

func TestHostedToolRunnerEnforcesCallOutputAndDeadlineLimits(t *testing.T) {
	owner := uuid.New()
	registry := NewHostedToolRegistry()
	executor := &fakeHostedExecutor{healthy: true, result: HostedToolResult{Output: json.RawMessage(`{"large":"output"}`)}}
	if err := registry.Register("file_search", executor); err != nil {
		t.Fatal(err)
	}
	runner, err := NewHostedToolRunner(registry, allowPolicy(owner, "gpt-test", "file_search"), owner, "resp_test", "gpt-test",
		HostedToolLimits{MaxCalls: 1, MaxOutputBytes: 4, CallTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Execute(context.Background(), "call_1", Tool{Type: "file_search"}, json.RawMessage(`{}`), nil); !errors.Is(err, ErrHostedToolOutputSize) {
		t.Fatalf("expected output limit, got %v", err)
	}
	if _, err := runner.Execute(context.Background(), "call_2", Tool{Type: "file_search"}, json.RawMessage(`{}`), nil); !errors.Is(err, ErrHostedToolLimit) {
		t.Fatalf("expected call limit, got %v", err)
	}

	blockedRegistry := NewHostedToolRegistry()
	blocked := make(chan struct{})
	if err := blockedRegistry.Register("code_interpreter", &fakeHostedExecutor{healthy: true, block: blocked}); err != nil {
		t.Fatal(err)
	}
	timed, _ := NewHostedToolRunner(blockedRegistry, allowPolicy(owner, "gpt-test", "code_interpreter"), owner, "resp_test", "gpt-test", HostedToolLimits{CallTimeout: time.Millisecond})
	if _, err := timed.Execute(context.Background(), "call_timeout", Tool{Type: "code_interpreter"}, json.RawMessage(`{}`), nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
}

func TestHostedToolRegistryCapabilitiesAreDeterministic(t *testing.T) {
	registry := NewHostedToolRegistry()
	for _, toolType := range []string{"code_interpreter", "web_search", "file_search"} {
		if err := registry.Register(toolType, &fakeHostedExecutor{healthy: true}); err != nil {
			t.Fatal(err)
		}
	}
	capabilities := registry.Capabilities(context.Background())
	if len(capabilities) != 3 || capabilities[0].Type != "web_search" || capabilities[1].Type != "file_search" || capabilities[2].Type != "code_interpreter" {
		t.Fatalf("unexpected capabilities: %#v", capabilities)
	}
}
