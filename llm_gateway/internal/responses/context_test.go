package responses

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestTruncateContextDisabledFails(t *testing.T) {
	_, err := TruncateContext([]ContextItem{{Tokens: 8}, {Tokens: 8, Current: true}}, 2, 12, TruncationDisabled, true)
	if !errors.Is(err, ErrContextLengthExceeded) {
		t.Fatalf("got %v", err)
	}
}

func TestTruncateContextAutoDropsOldestAndKeepsToolPair(t *testing.T) {
	items := []ContextItem{
		{Payload: json.RawMessage(`"old"`), Tokens: 4},
		{Payload: json.RawMessage(`"call"`), Tokens: 3, CallID: "call_1"},
		{Payload: json.RawMessage(`"output"`), Tokens: 3, CallID: "call_1"},
		{Payload: json.RawMessage(`"new"`), Tokens: 5, Current: true},
	}
	plan, err := TruncateContext(items, 1, 10, TruncationAuto, true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.DroppedItems != 3 || len(plan.Items) != 1 || !plan.Items[0].Current {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestTruncateContextAutoCannotDropCurrentInput(t *testing.T) {
	_, err := TruncateContext([]ContextItem{{Tokens: 20, Current: true}}, 1, 10, TruncationAuto, true)
	if !errors.Is(err, ErrContextLengthExceeded) {
		t.Fatalf("got %v", err)
	}
}

func TestFallbackTokenCountingReservesMargin(t *testing.T) {
	_, err := TruncateContext([]ContextItem{{Tokens: 10, Current: true}}, 0, 10, TruncationAuto, false)
	if !errors.Is(err, ErrContextLengthExceeded) {
		t.Fatal("expected safety reserve to fail closed")
	}
}

func TestAssembleTranslatedContextKeepsChainOrderAndOnlyCurrentInstructions(t *testing.T) {
	oldCall := "call_1"
	history := []Item{
		{Payload: json.RawMessage(`{"type":"message","role":"user","content":"first"}`), TokenCount: 2},
		{Payload: json.RawMessage(`{"type":"function_call","call_id":"call_1"}`), TokenCount: 2, CallID: &oldCall},
		{Payload: json.RawMessage(`{"type":"function_call_output","call_id":"call_1"}`), TokenCount: 2, CallID: &oldCall},
	}
	currentText := "latest"
	plan, err := AssembleTranslatedContext(history, Input{Set: true, String: &currentText}, json.RawMessage(`"new instructions"`), 100, TruncationDisabled, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 4 || string(plan.Items[0].Payload) != string(history[0].Payload) || !plan.Items[3].Current {
		t.Fatalf("unexpected order: %#v", plan)
	}
	for _, item := range plan.Items {
		if string(item.Payload) == `"old instructions"` {
			t.Fatal("historical instructions were inherited")
		}
	}
}
