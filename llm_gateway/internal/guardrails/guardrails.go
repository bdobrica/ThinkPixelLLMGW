// Package guardrails defines the gateway-owned port for optional content
// evaluation. Implementations are adapters to versioned wire contracts; they
// are not authorization services.
package guardrails

import "context"

type Stage string

const (
	StagePreModel  Stage = "pre_model"
	StagePostModel Stage = "post_model"
)

type Content struct {
	Type     string         `json:"type,omitempty"`
	Text     string         `json:"text,omitempty"`
	Messages []Message      `json:"messages,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Selection struct {
	Profile  string   `json:"profile,omitempty"`
	Policies []string `json:"policies,omitempty"`
}

type EvaluationRequest struct {
	RequestID  string         `json:"request_id"`
	Stage      Stage          `json:"stage"`
	TenantID   string         `json:"tenant_id,omitempty"`
	Guardrails Selection      `json:"guardrails,omitempty"`
	Content    Content        `json:"content"`
	Subject    map[string]any `json:"subject,omitempty"`
	Target     map[string]any `json:"target,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type Action string

const (
	ActionAllow   Action = "allow"
	ActionBlock   Action = "block"
	ActionRedact  Action = "redact"
	ActionMonitor Action = "monitor"
)

type Decision struct {
	Action             Action   `json:"action"`
	Reason             string   `json:"reason"`
	TransformedContent *Content `json:"transformed_content,omitempty"`
}

type Finding struct {
	Detector   string     `json:"detector"`
	Category   string     `json:"category"`
	Confidence float64    `json:"confidence"`
	Severity   string     `json:"severity,omitempty"`
	Locations  []Location `json:"locations,omitempty"`
}

type Location struct {
	Path  string `json:"path"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

type EvaluationResponse struct {
	EvaluationID    string         `json:"evaluation_id"`
	RequestID       string         `json:"request_id"`
	Decision        Decision       `json:"decision"`
	AppliedPolicies []string       `json:"applied_policies"`
	Findings        []Finding      `json:"findings"`
	Timing          map[string]any `json:"timing"`
}

type Evaluator interface {
	Evaluate(context.Context, EvaluationRequest) (*EvaluationResponse, error)
}
