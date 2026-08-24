package responses

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type flushBuffer struct {
	bytes.Buffer
	flushes int
}

func (b *flushBuffer) Flush() { b.flushes++ }

func TestSSEWriterProducesNamedOrderedFramesAndFlushes(t *testing.T) {
	output := &flushBuffer{}
	stream := NewSSEWriter(output)
	events := []StreamEvent{
		&ResponseEvent{EventBase: EventBase{Type: EventResponseCreated}, Response: Response{ID: "resp_1", Status: StatusInProgress}},
		&OutputItemEvent{EventBase: EventBase{Type: EventOutputItemAdded}, OutputIndex: 0, Item: OutputItem{ID: "msg_1", Type: "message"}},
		&ContentPartEvent{EventBase: EventBase{Type: EventContentPartAdded}, ItemID: "msg_1", OutputIndex: 0, ContentIndex: 0, Part: OutputContent{Type: "output_text"}},
		&TextDeltaEvent{EventBase: EventBase{Type: EventOutputTextDelta}, ItemID: "msg_1", OutputIndex: 0, ContentIndex: 0, Delta: "héllo\n世界"},
		&TextDoneEvent{EventBase: EventBase{Type: EventOutputTextDone}, ItemID: "msg_1", OutputIndex: 0, ContentIndex: 0, Text: "héllo\n世界"},
		&ContentPartEvent{EventBase: EventBase{Type: EventContentPartDone}, ItemID: "msg_1", OutputIndex: 0, ContentIndex: 0, Part: OutputContent{Type: "output_text", Text: "héllo\n世界"}},
		&OutputItemEvent{EventBase: EventBase{Type: EventOutputItemDone}, OutputIndex: 0, Item: OutputItem{ID: "msg_1", Type: "message", Status: StatusCompleted}},
		&ResponseEvent{EventBase: EventBase{Type: EventResponseCompleted}, Response: Response{ID: "resp_1", Status: StatusCompleted, Usage: &Usage{TotalTokens: 3}}},
	}
	for _, event := range events {
		if err := stream.WriteEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	if output.flushes != len(events) {
		t.Fatalf("flushes = %d, want %d", output.flushes, len(events))
	}

	scanner := bufio.NewScanner(strings.NewReader(output.String()))
	for index, want := range events {
		if !scanner.Scan() || scanner.Text() != "event: "+string(want.EventType()) {
			t.Fatalf("event line %d = %q", index, scanner.Text())
		}
		if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "data: ") {
			t.Fatalf("missing data line for event %d", index)
		}
		var envelope struct {
			Type           EventType `json:"type"`
			SequenceNumber int64     `json:"sequence_number"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(scanner.Text(), "data: ")), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Type != want.EventType() || envelope.SequenceNumber != int64(index) {
			t.Fatalf("event %d envelope = %#v", index, envelope)
		}
		if !scanner.Scan() || scanner.Text() != "" {
			t.Fatalf("event %d lacks SSE frame terminator", index)
		}
	}
	if !strings.Contains(output.String(), `"delta":"héllo\n世界"`) {
		t.Fatal("UTF-8 text or JSON newline boundary changed")
	}
}

func TestSSEWriterRejectsInvalidOrderingWithoutWriting(t *testing.T) {
	tests := []struct {
		name   string
		prefix []StreamEvent
		event  StreamEvent
	}{
		{name: "delta before create", event: &TextDeltaEvent{EventBase: EventBase{Type: EventOutputTextDelta}}},
		{name: "delta before item", prefix: []StreamEvent{createdEvent()}, event: &TextDeltaEvent{EventBase: EventBase{Type: EventOutputTextDelta}, ItemID: "msg_1"}},
		{name: "duplicate item index", prefix: []StreamEvent{createdEvent(), addedItem("msg_1", 0)}, event: addedItem("msg_2", 0)},
		{name: "nonterminal usage", prefix: []StreamEvent{createdEvent()}, event: &ResponseEvent{EventBase: EventBase{Type: EventResponseInProgress}, Response: Response{Status: StatusInProgress, Usage: &Usage{}}}},
		{name: "after terminal", prefix: []StreamEvent{createdEvent(), completedEvent()}, event: &ResponseEvent{EventBase: EventBase{Type: EventResponseInProgress}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			stream := NewSSEWriter(&output)
			for _, event := range test.prefix {
				if err := stream.WriteEvent(event); err != nil {
					t.Fatal(err)
				}
			}
			before := output.Len()
			if err := stream.WriteEvent(test.event); err == nil {
				t.Fatal("expected ordering error")
			}
			if output.Len() != before {
				t.Fatal("invalid event wrote bytes")
			}
		})
	}
}

func createdEvent() StreamEvent {
	return &ResponseEvent{EventBase: EventBase{Type: EventResponseCreated}, Response: Response{ID: "resp_1", Status: StatusInProgress}}
}
func completedEvent() StreamEvent {
	return &ResponseEvent{EventBase: EventBase{Type: EventResponseCompleted}, Response: Response{ID: "resp_1", Status: StatusCompleted}}
}
func addedItem(id string, index int) StreamEvent {
	return &OutputItemEvent{EventBase: EventBase{Type: EventOutputItemAdded}, OutputIndex: index, Item: OutputItem{ID: id, Type: "message"}}
}
