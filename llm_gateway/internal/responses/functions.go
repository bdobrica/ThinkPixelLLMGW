package responses

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// validateFunctionOutputs proves every client-provided output belongs to one
// unresolved call in the same tenant-owned response chain. Client functions
// are deliberately never executed by the gateway.
func (o *Orchestrator) validateFunctionOutputs(ctx context.Context, owner uuid.UUID, previousID string, current []InputItem) error {
	unresolved := map[string]struct{}{}
	resolved := map[string]struct{}{}
	if previousID != "" {
		_, items, err := o.Store.LoadChain(ctx, owner, previousID)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := applyFunctionItem(item.ItemType, item.CallID, item.Payload, unresolved, resolved, "previous_response_id"); err != nil {
				return err
			}
		}
	}
	for index, item := range current {
		callID := item.CallID
		if err := applyFunctionItem(item.Type, callIDPointer(callID), mustMarshal(item), unresolved, resolved, fmt.Sprintf("input[%d].call_id", index)); err != nil {
			return err
		}
	}
	return nil
}

func applyFunctionItem(itemType string, callID *string, _ json.RawMessage, unresolved, resolved map[string]struct{}, param string) error {
	if callID == nil || *callID == "" {
		return nil
	}
	switch itemType {
	case "function_call":
		if _, exists := unresolved[*callID]; exists {
			return invalid(param, "invalid_value", "duplicate function call_id %q", *callID)
		}
		if _, exists := resolved[*callID]; exists {
			return invalid(param, "invalid_value", "function call_id %q was already resolved", *callID)
		}
		unresolved[*callID] = struct{}{}
	case "function_call_output":
		if _, exists := resolved[*callID]; exists {
			return invalid(param, "invalid_value", "function call_id %q already has an output", *callID)
		}
		if _, exists := unresolved[*callID]; !exists {
			return invalid(param, "invalid_value", "function call_id %q does not match an unresolved call", *callID)
		}
		delete(unresolved, *callID)
		resolved[*callID] = struct{}{}
	}
	return nil
}

func normalizeFunctionCalls(response *Response, parallelAllowed bool) error {
	callCount := 0
	seen := map[string]struct{}{}
	for index := range response.Output {
		item := &response.Output[index]
		if item.Type != "function_call" {
			continue
		}
		callCount++
		if item.CallID == "" || !functionNamePattern.MatchString(item.Name) || !json.Valid([]byte(item.Arguments)) {
			return fmt.Errorf("function_call at output index %d is malformed", index)
		}
		if _, duplicate := seen[item.CallID]; duplicate {
			return fmt.Errorf("duplicate provider call_id %q", item.CallID)
		}
		seen[item.CallID] = struct{}{}
		id, err := (IDGenerator{}).NewItemID()
		if err != nil {
			return err
		}
		item.ID = id
		if item.Status == "" {
			item.Status = StatusCompleted
		}
	}
	if !parallelAllowed && callCount > 1 {
		return fmt.Errorf("provider returned %d calls with parallel_tool_calls=false", callCount)
	}
	return nil
}

func callIDPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func mustMarshal(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}
