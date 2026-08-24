package responses

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// SSEWriter serializes the Responses event protocol. It owns sequence numbers
// and rejects invalid lifecycle transitions before any bytes are written.
type SSEWriter struct {
	mu       sync.Mutex
	writer   io.Writer
	flusher  interface{ Flush() }
	next     int64
	created  bool
	terminal bool
	items    map[string]*streamItemState
	indices  map[int]string
}

type streamItemState struct {
	index int
	done  bool
	parts map[int]bool
}

func NewSSEWriter(writer io.Writer) *SSEWriter {
	stream := &SSEWriter{writer: writer, items: make(map[string]*streamItemState), indices: make(map[int]string)}
	if flusher, ok := writer.(interface{ Flush() }); ok {
		stream.flusher = flusher
	}
	return stream
}

// SetSSEHeaders applies headers required for an unbuffered event stream. It
// intentionally does not write a status code so callers can finish validation.
func SetSSEHeaders(header http.Header) {
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Accel-Buffering", "no")
}

func (s *SSEWriter) WriteEvent(event StreamEvent) error {
	if event == nil {
		return errors.New("nil Responses stream event")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writer == nil {
		return errors.New("nil Responses SSE writer")
	}
	if err := s.validate(event); err != nil {
		return err
	}
	setEventSequence(event, s.next)
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal %s event: %w", event.EventType(), err)
	}
	var frame bytes.Buffer
	fmt.Fprintf(&frame, "event: %s\n", event.EventType())
	frame.WriteString("data: ")
	frame.Write(payload)
	frame.WriteString("\n\n")
	if _, err := s.writer.Write(frame.Bytes()); err != nil {
		return fmt.Errorf("write %s event: %w", event.EventType(), err)
	}
	s.commit(event)
	s.next++
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return nil
}

func (s *SSEWriter) validate(event StreamEvent) error {
	typeName := event.EventType()
	if s.terminal {
		return fmt.Errorf("cannot emit %s after terminal Responses event", typeName)
	}
	if !s.created && typeName != EventResponseCreated {
		return fmt.Errorf("first Responses stream event must be %s", EventResponseCreated)
	}
	if s.created && typeName == EventResponseCreated {
		return errors.New("Responses stream already created")
	}
	switch value := event.(type) {
	case *ResponseEvent:
		if typeName != EventResponseCreated && typeName != EventResponseInProgress && !isTerminalEvent(typeName) {
			return fmt.Errorf("event type %s does not match response event payload", typeName)
		}
		terminal := isTerminalEvent(typeName)
		if !terminal && value.Response.Usage != nil {
			return fmt.Errorf("usage is only allowed on a terminal Responses event")
		}
		if !responseEventMatchesStatus(typeName, value.Response.Status) {
			return fmt.Errorf("event %s does not match response status %s", typeName, value.Response.Status)
		}
	case *OutputItemEvent:
		if typeName != EventOutputItemAdded && typeName != EventOutputItemDone {
			return fmt.Errorf("event type %s does not match output item payload", typeName)
		}
		if value.OutputIndex < 0 || value.Item.ID == "" {
			return errors.New("output item event requires a non-negative index and item id")
		}
		state, exists := s.items[value.Item.ID]
		if typeName == EventOutputItemAdded {
			if exists || s.indices[value.OutputIndex] != "" {
				return errors.New("output item was already added")
			}
		} else if !exists || state.index != value.OutputIndex || state.done {
			return errors.New("output item done requires a matching open item")
		}
	case *ContentPartEvent:
		if typeName != EventContentPartAdded && typeName != EventContentPartDone {
			return fmt.Errorf("event type %s does not match content part payload", typeName)
		}
		state, err := s.openItem(value.ItemID, value.OutputIndex)
		if err != nil {
			return err
		}
		_, exists := state.parts[value.ContentIndex]
		if value.ContentIndex < 0 || (typeName == EventContentPartAdded && exists) || (typeName == EventContentPartDone && (!exists || state.parts[value.ContentIndex])) {
			return errors.New("content part event is out of order")
		}
	case *TextDeltaEvent:
		if typeName != EventOutputTextDelta && typeName != EventRefusalDelta {
			return fmt.Errorf("event type %s does not match text delta payload", typeName)
		}
		if err := s.requireOpenPart(value.ItemID, value.OutputIndex, value.ContentIndex); err != nil {
			return err
		}
	case *TextDoneEvent:
		if typeName != EventOutputTextDone && typeName != EventRefusalDone {
			return fmt.Errorf("event type %s does not match text completion payload", typeName)
		}
		if err := s.requireOpenPart(value.ItemID, value.OutputIndex, value.ContentIndex); err != nil {
			return err
		}
	case *FunctionArgumentsEvent:
		if typeName != EventFunctionCallArgumentsDelta && typeName != EventFunctionCallArgumentsDone {
			return fmt.Errorf("event type %s does not match function arguments payload", typeName)
		}
		if _, err := s.openItem(value.ItemID, value.OutputIndex); err != nil {
			return err
		}
	case *ReasoningSummaryEvent:
		if typeName != EventReasoningSummaryTextDelta && typeName != EventReasoningSummaryTextDone {
			return fmt.Errorf("event type %s does not match reasoning summary payload", typeName)
		}
		if _, err := s.openItem(value.ItemID, value.OutputIndex); err != nil {
			return err
		}
	case *ErrorEvent:
		if typeName != EventError {
			return fmt.Errorf("event type %s does not match error payload", typeName)
		}
	default:
		return fmt.Errorf("unsupported Responses stream event %T", event)
	}
	return nil
}

func (s *SSEWriter) commit(event StreamEvent) {
	switch value := event.(type) {
	case *ResponseEvent:
		if value.Type == EventResponseCreated {
			s.created = true
		}
		if isTerminalEvent(value.Type) {
			s.terminal = true
		}
	case *OutputItemEvent:
		if value.Type == EventOutputItemAdded {
			s.items[value.Item.ID] = &streamItemState{index: value.OutputIndex, parts: make(map[int]bool)}
			s.indices[value.OutputIndex] = value.Item.ID
		} else {
			s.items[value.Item.ID].done = true
		}
	case *ContentPartEvent:
		if value.Type == EventContentPartAdded {
			s.items[value.ItemID].parts[value.ContentIndex] = false
		} else {
			s.items[value.ItemID].parts[value.ContentIndex] = true
		}
	case *ErrorEvent:
		s.terminal = true
	}
}

func (s *SSEWriter) openItem(itemID string, outputIndex int) (*streamItemState, error) {
	state, ok := s.items[itemID]
	if !ok || state.index != outputIndex || state.done {
		return nil, errors.New("event requires a matching open output item")
	}
	return state, nil
}

func (s *SSEWriter) requireOpenPart(itemID string, outputIndex, contentIndex int) error {
	state, err := s.openItem(itemID, outputIndex)
	if err != nil {
		return err
	}
	done, ok := state.parts[contentIndex]
	if !ok || done {
		return errors.New("event requires a matching open content part")
	}
	return nil
}

func setEventSequence(event StreamEvent, sequence int64) {
	switch value := event.(type) {
	case *ResponseEvent:
		value.SequenceNumberValue = sequence
	case *OutputItemEvent:
		value.SequenceNumberValue = sequence
	case *ContentPartEvent:
		value.SequenceNumberValue = sequence
	case *TextDeltaEvent:
		value.SequenceNumberValue = sequence
	case *TextDoneEvent:
		value.SequenceNumberValue = sequence
	case *FunctionArgumentsEvent:
		value.SequenceNumberValue = sequence
	case *ReasoningSummaryEvent:
		value.SequenceNumberValue = sequence
	case *ErrorEvent:
		value.SequenceNumberValue = sequence
	}
}

func isTerminalEvent(eventType EventType) bool {
	return eventType == EventResponseCompleted || eventType == EventResponseIncomplete || eventType == EventResponseFailed
}

func responseEventMatchesStatus(eventType EventType, status Status) bool {
	switch eventType {
	case EventResponseCreated, EventResponseInProgress:
		return status == StatusInProgress
	case EventResponseCompleted:
		return status == StatusCompleted
	case EventResponseIncomplete:
		return status == StatusIncomplete
	case EventResponseFailed:
		return status == StatusFailed
	default:
		return false
	}
}
