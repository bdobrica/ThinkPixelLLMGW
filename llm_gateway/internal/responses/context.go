package responses

import (
	"encoding/json"
	"errors"
)

var ErrContextLengthExceeded = errors.New("response input exceeds the model context window")

// ContextItem is an item prepared for a translated Responses provider. Current
// request items and instructions are protected; CallID makes a function call
// and its output an indivisible truncation group.
type ContextItem struct {
	Payload  json.RawMessage
	Tokens   int
	CallID   string
	Current  bool
	Required bool
}

type ContextPlan struct {
	Items         []ContextItem
	Tokens        int
	DroppedItems  int
	SafetyReserve int
}

// AssembleTranslatedContext converts a Store.LoadChain result plus the current
// request into provider-neutral ordered context. Historical request envelopes
// are intentionally ignored so their instructions cannot be inherited.
func AssembleTranslatedContext(history []Item, current Input, currentInstructions json.RawMessage, maxContextTokens int, mode Truncation, exactCounting bool) (ContextPlan, error) {
	items := make([]ContextItem, 0, len(history)+len(current.Items)+1)
	for _, item := range history {
		callID := ""
		if item.CallID != nil {
			callID = *item.CallID
		}
		items = append(items, ContextItem{Payload: append(json.RawMessage(nil), item.Payload...), Tokens: item.TokenCount, CallID: callID})
	}
	currentItems, err := persistenceInputItems("", current)
	if err != nil {
		return ContextPlan{}, err
	}
	for _, item := range currentItems {
		callID := ""
		if item.CallID != nil {
			callID = *item.CallID
		}
		items = append(items, ContextItem{Payload: item.Payload, Tokens: item.TokenCount, CallID: callID, Current: true})
	}
	return TruncateContext(items, EstimateTokens(currentInstructions), maxContextTokens, mode, exactCounting)
}

// TruncateContext applies the pinned Responses behavior. The fallback token
// estimate reserves ten percent of the model window and fails closed.
func TruncateContext(items []ContextItem, instructionTokens, maxContextTokens int, mode Truncation, exactCounting bool) (ContextPlan, error) {
	if maxContextTokens <= 0 {
		return ContextPlan{}, ErrContextLengthExceeded
	}
	reserve := 0
	if !exactCounting {
		reserve = maxContextTokens / 10
		if reserve < 1 {
			reserve = 1
		}
	}
	budget := maxContextTokens - reserve
	total := instructionTokens
	for i := range items {
		if items[i].Tokens <= 0 {
			items[i].Tokens = EstimateTokens(items[i].Payload)
		}
		total += items[i].Tokens
	}
	if total <= budget {
		return ContextPlan{Items: append([]ContextItem(nil), items...), Tokens: total, SafetyReserve: reserve}, nil
	}
	if mode == "" || mode == TruncationDisabled {
		return ContextPlan{}, ErrContextLengthExceeded
	}
	keep := make([]bool, len(items))
	for i := range keep {
		keep[i] = true
	}
	for i := 0; i < len(items) && total > budget; i++ {
		if !keep[i] || items[i].Current || items[i].Required {
			continue
		}
		group := []int{i}
		if items[i].CallID != "" {
			group = group[:0]
			blocked := false
			for j := range items {
				if keep[j] && items[j].CallID == items[i].CallID {
					if items[j].Current || items[j].Required {
						blocked = true
						break
					}
					group = append(group, j)
				}
			}
			if blocked {
				continue
			}
		}
		for _, index := range group {
			if keep[index] {
				keep[index] = false
				total -= items[index].Tokens
			}
		}
	}
	if total > budget {
		return ContextPlan{}, ErrContextLengthExceeded
	}
	result := make([]ContextItem, 0, len(items))
	for i := range items {
		if keep[i] {
			result = append(result, items[i])
		}
	}
	return ContextPlan{Items: result, Tokens: total, DroppedItems: len(items) - len(result), SafetyReserve: reserve}, nil
}
