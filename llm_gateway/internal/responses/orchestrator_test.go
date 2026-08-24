package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

type fakeStateStore struct {
	predecessor *Record
	created     CreateRecord
	items       []Item
	correlation string
	terminal    TerminalUpdate
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

func containsJSONField(payload []byte, key string) bool {
	var value map[string]any
	_ = json.Unmarshal(payload, &value)
	_, ok := value[key]
	return ok
}
