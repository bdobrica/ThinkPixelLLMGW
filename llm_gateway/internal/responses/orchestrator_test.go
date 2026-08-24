package responses

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeStateStore struct {
	predecessor *Record
	chainItems  []Item
	created     CreateRecord
	items       []Item
	correlation string
	terminal    TerminalUpdate
}

func (s *fakeStateStore) LoadChain(context.Context, uuid.UUID, string) ([]Record, []Item, error) {
	if s.predecessor == nil {
		return nil, nil, ErrNotFound
	}
	return []Record{*s.predecessor}, s.chainItems, nil
}

func (s *fakeStateStore) Create(_ context.Context, record CreateRecord, items []Item) error {
	s.created, s.items = record, items
	return nil
}
func (s *fakeStateStore) Get(context.Context, uuid.UUID, string) (*Record, error) {
	if s.predecessor == nil {
		return nil, ErrNotFound
	}
	return s.predecessor, nil
}
func (s *fakeStateStore) MarkInProgress(context.Context, uuid.UUID, string) error { return nil }
func (s *fakeStateStore) SetProviderCorrelationID(_ context.Context, _ uuid.UUID, _ string, value string) error {
	s.correlation = value
	return nil
}
func (s *fakeStateStore) Complete(_ context.Context, _ uuid.UUID, _ string, update TerminalUpdate) error {
	s.terminal = update
	return nil
}
func (s *fakeStateStore) EncryptOpaquePayload(value []byte) (*string, error) {
	sealed := "sealed:" + string(value)
	return &sealed, nil
}

type fakeNativeTransport struct {
	payload []byte
	body    []byte
}

func (t *fakeNativeTransport) CreateResponse(_ context.Context, payload []byte, _ bool) (*TransportResponse, error) {
	t.payload = append([]byte(nil), payload...)
	return &TransportResponse{StatusCode: 200, Body: t.body}, nil
}

func TestCreateNativeMapsGatewayAndProviderStateIDs(t *testing.T) {
	upstreamPrevious := "resp_upstream_previous"
	store := &fakeStateStore{predecessor: &Record{ProviderCorrelationID: &upstreamPrevious}}
	transport := &fakeNativeTransport{body: []byte(`{"id":"resp_upstream_new","object":"response","created_at":1,"status":"completed","error":null,"incomplete_details":null,"model":"gpt-test","previous_response_id":"resp_upstream_previous","output":[],"tools":[],"tool_choice":"auto","parallel_tool_calls":true,"truncation":"disabled","usage":{"input_tokens":1,"input_tokens_details":{"cached_tokens":0},"output_tokens":1,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":2},"metadata":{},"store":true,"background":false,"max_output_tokens":null}`)}
	input := "hello"
	request := CreateRequest{Model: "alias", Input: Input{Set: true, String: &input}, PreviousResponseID: "resp_gateway_previous"}
	result, _, err := (&Orchestrator{Store: store}).CreateNative(context.Background(), request, CreateOptions{Owner: uuid.New(), ProviderModel: "gpt-test", Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	var sent CreateRequest
	if err := json.Unmarshal(transport.payload, &sent); err != nil {
		t.Fatal(err)
	}
	if sent.PreviousResponseID != upstreamPrevious {
		t.Fatalf("provider got %q", sent.PreviousResponseID)
	}
	if result.ID == "resp_upstream_new" || result.PreviousResponseID == nil || *result.PreviousResponseID != "resp_gateway_previous" {
		t.Fatalf("public IDs leaked or changed: %#v", result)
	}
	if store.correlation != "resp_upstream_new" || store.created.PreviousResponseID == nil || *store.created.PreviousResponseID != "resp_gateway_previous" {
		t.Fatalf("mapping not persisted: %#v", store)
	}
	if store.terminal.Status != StatusCompleted || len(store.items) != 1 {
		t.Fatalf("unexpected persistence: %#v", store)
	}
}

func TestCreateNativeDoesNotInheritPredecessorInstructions(t *testing.T) {
	upstreamPrevious := "resp_upstream_previous"
	store := &fakeStateStore{predecessor: &Record{ProviderCorrelationID: &upstreamPrevious, Request: json.RawMessage(`{"instructions":"old"}`)}}
	transport := &fakeNativeTransport{body: []byte(`{"id":"resp_new","object":"response","created_at":1,"status":"completed","error":null,"incomplete_details":null,"model":"gpt-test","previous_response_id":"resp_upstream_previous","output":[],"tools":[],"tool_choice":"auto","parallel_tool_calls":true,"truncation":"disabled","usage":null,"metadata":{},"store":true,"background":false,"max_output_tokens":null}`)}
	input := "next"
	request := CreateRequest{Model: "gpt-test", Input: Input{Set: true, String: &input}, PreviousResponseID: "resp_gateway_previous"}
	_, _, err := (&Orchestrator{Store: store}).CreateNative(context.Background(), request, CreateOptions{Owner: uuid.New(), ProviderModel: "gpt-test", Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	var sent map[string]any
	if err := json.Unmarshal(transport.payload, &sent); err != nil {
		t.Fatal(err)
	}
	if _, exists := sent["instructions"]; exists {
		t.Fatalf("predecessor instructions were inherited: %s", transport.payload)
	}
}

func TestCreateNativeEncryptsOpaqueReasoningAtRest(t *testing.T) {
	store := &fakeStateStore{}
	transport := &fakeNativeTransport{body: []byte(`{"id":"resp_new","object":"response","created_at":1,"status":"completed","error":null,"incomplete_details":null,"model":"gpt-test","previous_response_id":null,"output":[{"type":"reasoning","id":"item_reasoning","status":"completed","summary":[],"encrypted_content":"provider-ciphertext"}],"tools":[],"tool_choice":"auto","parallel_tool_calls":true,"truncation":"disabled","usage":null,"metadata":{},"store":true,"background":false,"max_output_tokens":null}`)}
	input := "think"
	result, _, err := (&Orchestrator{Store: store}).CreateNative(context.Background(), CreateRequest{Model: "gpt-test", Input: Input{Set: true, String: &input}}, CreateOptions{Owner: uuid.New(), ProviderModel: "gpt-test", Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output[0].EncryptedContent != "provider-ciphertext" {
		t.Fatal("client response lost provider opaque reasoning")
	}
	item := store.terminal.Items[0]
	if item.EncryptedPayload == nil || *item.EncryptedPayload != "sealed:provider-ciphertext" {
		t.Fatalf("opaque reasoning was not sealed: %#v", item)
	}
	if string(item.Payload) == "" || json.Valid(item.Payload) == false || containsJSONField(item.Payload, "encrypted_content") {
		t.Fatalf("opaque reasoning leaked into JSON payload: %s", item.Payload)
	}
}

func TestCreateNativeCorrelatesFunctionOutputToPredecessor(t *testing.T) {
	upstreamPrevious := "resp_upstream_previous"
	callID := "call_weather"
	store := &fakeStateStore{
		predecessor: &Record{ProviderCorrelationID: &upstreamPrevious},
		chainItems:  []Item{{ItemType: "function_call", CallID: &callID, Payload: json.RawMessage(`{"type":"function_call","call_id":"call_weather"}`)}},
	}
	transport := &fakeNativeTransport{body: []byte(`{"id":"resp_new","object":"response","created_at":1,"status":"completed","error":null,"incomplete_details":null,"model":"gpt-test","previous_response_id":"resp_upstream_previous","output":[],"tools":[],"tool_choice":"auto","parallel_tool_calls":true,"truncation":"disabled","usage":null,"metadata":{},"store":true,"background":false,"max_output_tokens":null}`)}
	request := CreateRequest{Model: "gpt-test", PreviousResponseID: "resp_gateway_previous", Input: Input{Set: true, Items: []InputItem{{Type: "function_call_output", CallID: callID, Output: json.RawMessage(`{"temperature":21}`)}}}}
	if _, _, err := (&Orchestrator{Store: store}).CreateNative(context.Background(), request, CreateOptions{Owner: uuid.New(), ProviderModel: "gpt-test", Transport: transport}); err != nil {
		t.Fatal(err)
	}
}

func TestCreateNativeRejectsUnknownAndDuplicateFunctionOutputs(t *testing.T) {
	upstreamPrevious := "resp_upstream_previous"
	callID := "call_weather"
	store := &fakeStateStore{
		predecessor: &Record{ProviderCorrelationID: &upstreamPrevious},
		chainItems:  []Item{{ItemType: "function_call", CallID: &callID}},
	}
	for _, test := range []struct {
		name  string
		items []InputItem
	}{
		{"unknown", []InputItem{{Type: "function_call_output", CallID: "call_unknown", Output: json.RawMessage(`"nope"`)}}},
		{"duplicate", []InputItem{
			{Type: "function_call_output", CallID: callID, Output: json.RawMessage(`"first"`)},
			{Type: "function_call_output", CallID: callID, Output: json.RawMessage(`"second"`)},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := CreateRequest{Model: "gpt-test", PreviousResponseID: "resp_gateway_previous", Input: Input{Set: true, Items: test.items}}
			_, _, err := (&Orchestrator{Store: store}).CreateNative(context.Background(), request, CreateOptions{Owner: uuid.New(), ProviderModel: "gpt-test", Transport: &fakeNativeTransport{}})
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Param == "" {
				t.Fatalf("expected correlated validation error, got %v", err)
			}
		})
	}
}

func TestNormalizeFunctionCallsPreservesOrderAndAssignsStableItems(t *testing.T) {
	response := Response{Output: []OutputItem{
		{Type: "function_call", ID: "provider_item_1", CallID: "call_1", Name: "first", Arguments: `{"x":1}`},
		{Type: "function_call", ID: "provider_item_2", CallID: "call_2", Name: "second", Arguments: `{"x":2}`},
	}}
	if err := normalizeFunctionCalls(&response, true); err != nil {
		t.Fatal(err)
	}
	if response.Output[0].CallID != "call_1" || response.Output[1].CallID != "call_2" || response.Output[0].ID == "provider_item_1" || response.Output[0].ID == response.Output[1].ID {
		t.Fatalf("function calls were reordered or IDs were not normalized: %#v", response.Output)
	}
	if err := normalizeFunctionCalls(&Response{Output: response.Output}, false); err == nil {
		t.Fatal("expected parallel call rejection")
	}
}

func containsJSONField(payload []byte, key string) bool {
	var value map[string]any
	_ = json.Unmarshal(payload, &value)
	_, ok := value[key]
	return ok
}
