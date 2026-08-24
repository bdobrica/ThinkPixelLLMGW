package responses

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrHostedToolDisabled   = errors.New("hosted tool is not enabled")
	ErrHostedToolLimit      = errors.New("hosted tool call limit exceeded")
	ErrHostedToolOutputSize = errors.New("hosted tool output exceeds limit")
	ErrHostedToolCallReuse  = errors.New("hosted tool call ID reused with different input")
)

// HostedToolDescriptor is safe to expose through capability and readiness
// reporting. It deliberately contains no backend credentials or arguments.
type HostedToolDescriptor struct {
	Type        string
	Description string
	Healthy     bool
}

type HostedToolRequest struct {
	OwnerID    uuid.UUID
	ResponseID string
	CallID     string
	Model      string
	Tool       Tool
	Arguments  json.RawMessage
}

type HostedToolUsage struct {
	Calls       int   `json:"calls"`
	InputBytes  int64 `json:"input_bytes"`
	OutputBytes int64 `json:"output_bytes"`
	CostNanoUSD int64 `json:"cost_nano_usd"`
}

type HostedToolResult struct {
	Output json.RawMessage
	Usage  HostedToolUsage
}

type HostedToolEvent struct {
	Type       string
	ToolType   string
	ResponseID string
	CallID     string
	Message    string
	Usage      *HostedToolUsage
}

type HostedToolExecutor interface {
	Descriptor(context.Context) HostedToolDescriptor
	Validate(json.RawMessage) error
	Execute(context.Context, HostedToolRequest, func(HostedToolEvent)) (HostedToolResult, error)
}

// HostedToolRegistry contains installed executors. Registration alone never
// enables a tool; HostedToolPolicy must independently authorize every scope.
type HostedToolRegistry struct {
	mu        sync.RWMutex
	executors map[string]HostedToolExecutor
}

func NewHostedToolRegistry() *HostedToolRegistry {
	return &HostedToolRegistry{executors: make(map[string]HostedToolExecutor)}
}

func (r *HostedToolRegistry) Register(toolType string, executor HostedToolExecutor) error {
	if r == nil || executor == nil || !isHostedToolType(toolType) {
		return fmt.Errorf("invalid hosted tool registration %q", toolType)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.executors[toolType]; exists {
		return fmt.Errorf("hosted tool %q is already registered", toolType)
	}
	r.executors[toolType] = executor
	return nil
}

func (r *HostedToolRegistry) Capabilities(ctx context.Context) []HostedToolDescriptor {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]HostedToolDescriptor, 0, len(r.executors))
	for _, toolType := range []string{"web_search", "file_search", "code_interpreter"} {
		if executor, ok := r.executors[toolType]; ok {
			descriptor := executor.Descriptor(ctx)
			descriptor.Type = toolType
			result = append(result, descriptor)
		}
	}
	return result
}

func (r *HostedToolRegistry) executor(toolType string) (HostedToolExecutor, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	executor, ok := r.executors[canonicalHostedToolType(toolType)]
	return executor, ok
}

// HostedToolPolicy is deliberately default-deny. A call must be present in
// the deployment, model, and API-key allowlists to run.
type HostedToolPolicy struct {
	DeploymentAllow map[string]bool
	ModelAllow      map[string]map[string]bool
	APIKeyAllow     map[uuid.UUID]map[string]bool
}

func (p HostedToolPolicy) Allows(owner uuid.UUID, model, toolType string) bool {
	toolType = canonicalHostedToolType(toolType)
	return p.DeploymentAllow[toolType] && p.ModelAllow[model][toolType] && p.APIKeyAllow[owner][toolType]
}

type HostedToolLimits struct {
	MaxCalls       int
	MaxConcurrency int
	MaxInputBytes  int
	MaxOutputBytes int
	CallTimeout    time.Duration
}

func (l HostedToolLimits) normalized() HostedToolLimits {
	if l.MaxCalls <= 0 {
		l.MaxCalls = 8
	}
	if l.MaxConcurrency <= 0 {
		l.MaxConcurrency = 1
	}
	if l.MaxInputBytes <= 0 {
		l.MaxInputBytes = 64 << 10
	}
	if l.MaxOutputBytes <= 0 {
		l.MaxOutputBytes = 1 << 20
	}
	if l.CallTimeout <= 0 {
		l.CallTimeout = 30 * time.Second
	}
	return l
}

type hostedToolCall struct {
	digest [32]byte
	result HostedToolResult
	err    error
	done   chan struct{}
}

// HostedToolRunner owns limits and idempotency for exactly one response.
type HostedToolRunner struct {
	registry   *HostedToolRegistry
	policy     HostedToolPolicy
	owner      uuid.UUID
	responseID string
	model      string
	limits     HostedToolLimits
	semaphore  chan struct{}

	mu    sync.Mutex
	calls map[string]*hostedToolCall
}

func NewHostedToolRunner(registry *HostedToolRegistry, policy HostedToolPolicy, owner uuid.UUID, responseID, model string, limits HostedToolLimits) (*HostedToolRunner, error) {
	if registry == nil || owner == uuid.Nil || responseID == "" || model == "" {
		return nil, errors.New("invalid hosted tool runner configuration")
	}
	limits = limits.normalized()
	return &HostedToolRunner{registry: registry, policy: policy, owner: owner, responseID: responseID, model: model,
		limits: limits, semaphore: make(chan struct{}, limits.MaxConcurrency), calls: make(map[string]*hostedToolCall)}, nil
}

func (r *HostedToolRunner) Execute(ctx context.Context, callID string, tool Tool, arguments json.RawMessage, emit func(HostedToolEvent)) (HostedToolResult, error) {
	toolType := canonicalHostedToolType(tool.Type)
	if callID == "" || !isHostedToolType(toolType) || !json.Valid(arguments) || bytes.Equal(bytes.TrimSpace(arguments), []byte("null")) {
		return HostedToolResult{}, errors.New("invalid hosted tool call")
	}
	if len(arguments) > r.limits.MaxInputBytes {
		return HostedToolResult{}, fmt.Errorf("hosted tool input exceeds %d bytes", r.limits.MaxInputBytes)
	}
	if !r.policy.Allows(r.owner, r.model, toolType) {
		return HostedToolResult{}, ErrHostedToolDisabled
	}
	executor, ok := r.registry.executor(toolType)
	if !ok || !executor.Descriptor(ctx).Healthy {
		return HostedToolResult{}, ErrHostedToolDisabled
	}
	if err := executor.Validate(arguments); err != nil {
		return HostedToolResult{}, fmt.Errorf("validate hosted tool %s: %w", toolType, err)
	}

	digest := sha256.Sum256(append([]byte(toolType+"\x00"), arguments...))
	call, leader, err := r.reserve(callID, digest)
	if err != nil {
		return HostedToolResult{}, err
	}
	if !leader {
		select {
		case <-call.done:
			return call.result, call.err
		case <-ctx.Done():
			return HostedToolResult{}, ctx.Err()
		}
	}

	result, runErr := r.run(ctx, executor, callID, tool, arguments, emit)
	r.mu.Lock()
	call.result, call.err = result, runErr
	close(call.done)
	r.mu.Unlock()
	return result, runErr
}

func (r *HostedToolRunner) reserve(callID string, digest [32]byte) (*hostedToolCall, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.calls[callID]; ok {
		if existing.digest != digest {
			return nil, false, ErrHostedToolCallReuse
		}
		return existing, false, nil
	}
	if len(r.calls) >= r.limits.MaxCalls {
		return nil, false, ErrHostedToolLimit
	}
	call := &hostedToolCall{digest: digest, done: make(chan struct{})}
	r.calls[callID] = call
	return call, true, nil
}

func (r *HostedToolRunner) run(ctx context.Context, executor HostedToolExecutor, callID string, tool Tool, arguments json.RawMessage, emit func(HostedToolEvent)) (HostedToolResult, error) {
	select {
	case r.semaphore <- struct{}{}:
		defer func() { <-r.semaphore }()
	case <-ctx.Done():
		return HostedToolResult{}, ctx.Err()
	}
	callCtx, cancel := context.WithTimeout(ctx, r.limits.CallTimeout)
	defer cancel()
	event := func(kind, message string, usage *HostedToolUsage) {
		if emit != nil {
			emit(HostedToolEvent{Type: kind, ToolType: canonicalHostedToolType(tool.Type), ResponseID: r.responseID, CallID: callID, Message: message, Usage: usage})
		}
	}
	event("tool.execution.started", "", nil)
	request := HostedToolRequest{OwnerID: r.owner, ResponseID: r.responseID, CallID: callID, Model: r.model, Tool: tool, Arguments: arguments}
	result, err := executor.Execute(callCtx, request, func(update HostedToolEvent) {
		event("tool.execution.progress", update.Message, update.Usage)
	})
	if err != nil {
		event("tool.execution.failed", safeToolError(err), nil)
		return HostedToolResult{}, err
	}
	if len(result.Output) > r.limits.MaxOutputBytes {
		event("tool.execution.failed", ErrHostedToolOutputSize.Error(), nil)
		return HostedToolResult{}, ErrHostedToolOutputSize
	}
	if !json.Valid(result.Output) {
		err = errors.New("hosted tool returned invalid JSON")
		event("tool.execution.failed", err.Error(), nil)
		return HostedToolResult{}, err
	}
	result.Usage.Calls = 1
	result.Usage.InputBytes = int64(len(arguments))
	result.Usage.OutputBytes = int64(len(result.Output))
	event("tool.execution.completed", "", &result.Usage)
	return result, nil
}

func canonicalHostedToolType(toolType string) string {
	if toolType == "web_search_preview" {
		return "web_search"
	}
	return toolType
}

func isHostedToolType(toolType string) bool {
	switch canonicalHostedToolType(toolType) {
	case "web_search", "file_search", "code_interpreter":
		return true
	default:
		return false
	}
}

func safeToolError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "hosted tool deadline exceeded"
	}
	if errors.Is(err, context.Canceled) {
		return "hosted tool cancelled"
	}
	return "hosted tool execution failed"
}
