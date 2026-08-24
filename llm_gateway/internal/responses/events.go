package responses

import (
	"encoding/json"
	"fmt"
)

type EventType string

const (
	EventResponseCreated            EventType = "response.created"
	EventResponseInProgress         EventType = "response.in_progress"
	EventResponseCompleted          EventType = "response.completed"
	EventResponseIncomplete         EventType = "response.incomplete"
	EventResponseFailed             EventType = "response.failed"
	EventOutputItemAdded            EventType = "response.output_item.added"
	EventOutputItemDone             EventType = "response.output_item.done"
	EventContentPartAdded           EventType = "response.content_part.added"
	EventContentPartDone            EventType = "response.content_part.done"
	EventOutputTextDelta            EventType = "response.output_text.delta"
	EventOutputTextDone             EventType = "response.output_text.done"
	EventRefusalDelta               EventType = "response.refusal.delta"
	EventRefusalDone                EventType = "response.refusal.done"
	EventFunctionCallArgumentsDelta EventType = "response.function_call_arguments.delta"
	EventFunctionCallArgumentsDone  EventType = "response.function_call_arguments.done"
	EventReasoningSummaryTextDelta  EventType = "response.reasoning_summary_text.delta"
	EventReasoningSummaryTextDone   EventType = "response.reasoning_summary_text.done"
	EventError                      EventType = "error"
)

type StreamEvent interface {
	EventType() EventType
	SequenceNumber() int64
}

type EventBase struct {
	Type                EventType `json:"type"`
	SequenceNumberValue int64     `json:"sequence_number"`
}

func (e EventBase) EventType() EventType  { return e.Type }
func (e EventBase) SequenceNumber() int64 { return e.SequenceNumberValue }

type ResponseEvent struct {
	EventBase
	Response Response `json:"response"`
}

type OutputItemEvent struct {
	EventBase
	OutputIndex int        `json:"output_index"`
	Item        OutputItem `json:"item"`
}

type ContentPartEvent struct {
	EventBase
	ItemID       string        `json:"item_id"`
	OutputIndex  int           `json:"output_index"`
	ContentIndex int           `json:"content_index"`
	Part         OutputContent `json:"part"`
}

type TextDeltaEvent struct {
	EventBase
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

type TextDoneEvent struct {
	EventBase
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Text         string `json:"text,omitempty"`
	Refusal      string `json:"refusal,omitempty"`
}

type FunctionArgumentsEvent struct {
	EventBase
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
	Delta       string `json:"delta,omitempty"`
	Arguments   string `json:"arguments,omitempty"`
}

type ReasoningSummaryEvent struct {
	EventBase
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	SummaryIndex int    `json:"summary_index"`
	Delta        string `json:"delta,omitempty"`
	Text         string `json:"text,omitempty"`
}

type ErrorEvent struct {
	EventBase
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Param   *string `json:"param"`
}

// DecodeStreamEvent decodes only the event set promised by this contract
// snapshot. Unknown event types must be added deliberately with fixtures.
func DecodeStreamEvent(data []byte) (StreamEvent, error) {
	var discriminator struct {
		Type EventType `json:"type"`
	}
	if err := json.Unmarshal(data, &discriminator); err != nil {
		return nil, fmt.Errorf("invalid Responses stream event: %w", err)
	}
	var event StreamEvent
	switch discriminator.Type {
	case EventResponseCreated, EventResponseInProgress, EventResponseCompleted, EventResponseIncomplete, EventResponseFailed:
		event = &ResponseEvent{}
	case EventOutputItemAdded, EventOutputItemDone:
		event = &OutputItemEvent{}
	case EventContentPartAdded, EventContentPartDone:
		event = &ContentPartEvent{}
	case EventOutputTextDelta, EventRefusalDelta:
		event = &TextDeltaEvent{}
	case EventOutputTextDone, EventRefusalDone:
		event = &TextDoneEvent{}
	case EventFunctionCallArgumentsDelta, EventFunctionCallArgumentsDone:
		event = &FunctionArgumentsEvent{}
	case EventReasoningSummaryTextDelta, EventReasoningSummaryTextDone:
		event = &ReasoningSummaryEvent{}
	case EventError:
		event = &ErrorEvent{}
	default:
		return nil, fmt.Errorf("unsupported Responses stream event type %q in contract snapshot %s", discriminator.Type, ContractSnapshot)
	}
	if err := json.Unmarshal(data, event); err != nil {
		return nil, fmt.Errorf("invalid %s event: %w", discriminator.Type, err)
	}
	if event.SequenceNumber() < 0 {
		return nil, fmt.Errorf("invalid %s sequence_number", discriminator.Type)
	}
	return event, nil
}
