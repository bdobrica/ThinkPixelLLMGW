package responses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"
)

// NativeTransport is the Responses-specific provider boundary. It deliberately
// does not share Chat Completions request or response types.
type NativeTransport interface {
	CreateResponse(context.Context, []byte, bool) (*TransportResponse, error)
}

type TransportResponse struct {
	StatusCode int
	Body       []byte
}

type StateStore interface {
	Create(context.Context, CreateRecord, []Item) error
	Get(context.Context, uuid.UUID, string) (*Record, error)
	MarkInProgress(context.Context, uuid.UUID, string) error
	SetProviderCorrelationID(context.Context, uuid.UUID, string, string) error
	Complete(context.Context, uuid.UUID, string, TerminalUpdate) error
	EncryptOpaquePayload([]byte) (*string, error)
}

type Orchestrator struct {
	Store StateStore
}

type CreateOptions struct {
	Owner         uuid.UUID
	ProviderModel string
	Transport     NativeTransport
}

// CreateNative creates a gateway-owned response envelope while delegating
// model state to a native Responses provider. Public gateway IDs never expose
// or accept an upstream response ID.
func (o *Orchestrator) CreateNative(ctx context.Context, request CreateRequest, options CreateOptions) (*Response, int, error) {
	if o == nil || o.Store == nil || options.Transport == nil || options.Owner == uuid.Nil {
		return nil, 0, errors.New("invalid Responses orchestrator configuration")
	}
	if request.Stream {
		return nil, 0, invalid("stream", "unsupported_parameter", "Responses streaming is not enabled until the streaming state machine is installed")
	}
	if request.Background {
		return nil, 0, invalid("background", "unsupported_parameter", "background Responses are not enabled until durable background execution is installed")
	}
	applyDefaults(&request)
	publicPrevious := request.PreviousResponseID
	if publicPrevious != "" {
		predecessor, err := o.Store.Get(ctx, options.Owner, publicPrevious)
		if err != nil {
			return nil, 0, err
		}
		if predecessor.ProviderCorrelationID == nil || *predecessor.ProviderCorrelationID == "" {
			return nil, 0, invalid("previous_response_id", "invalid_value", "previous response cannot be continued on the selected provider")
		}
		request.PreviousResponseID = *predecessor.ProviderCorrelationID
	}
	request.Model = options.ProviderModel
	providerPayload, err := json.Marshal(request)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal provider Responses request: %w", err)
	}
	storedRequest := request
	storedRequest.PreviousResponseID = publicPrevious
	storedPayload, err := json.Marshal(storedRequest)
	if err != nil {
		return nil, 0, err
	}
	responseID, err := (IDGenerator{}).NewResponseID()
	if err != nil {
		return nil, 0, err
	}
	stored := request.Store == nil || *request.Store
	var previous *string
	if publicPrevious != "" {
		previous = &publicPrevious
	}
	inputItems, err := persistenceInputItems(responseID, request.Input)
	if err != nil {
		return nil, 0, err
	}
	if err := encryptReasoningItems(o.Store, inputItems); err != nil {
		return nil, 0, err
	}
	if err := o.Store.Create(ctx, CreateRecord{ID: responseID, APIKeyID: options.Owner, PreviousResponseID: previous,
		Model: options.ProviderModel, Stored: stored, Request: storedPayload}, inputItems); err != nil {
		return nil, 0, err
	}
	if err := o.Store.MarkInProgress(ctx, options.Owner, responseID); err != nil {
		return nil, 0, err
	}
	providerResponse, err := options.Transport.CreateResponse(ctx, providerPayload, false)
	if err != nil {
		failure := json.RawMessage(`{"code":"provider_error","message":"provider request failed"}`)
		_ = o.Store.Complete(ctx, options.Owner, responseID, TerminalUpdate{Status: StatusFailed, Error: failure})
		return nil, 0, err
	}
	if providerResponse.StatusCode < 200 || providerResponse.StatusCode >= 300 {
		failure := json.RawMessage(`{"code":"provider_error","message":"provider rejected the request"}`)
		_ = o.Store.Complete(ctx, options.Owner, responseID, TerminalUpdate{Status: StatusFailed, Error: failure})
		return nil, providerResponse.StatusCode, &UpstreamError{StatusCode: providerResponse.StatusCode, Body: providerResponse.Body}
	}
	var result Response
	if err := json.Unmarshal(providerResponse.Body, &result); err != nil {
		failure := json.RawMessage(`{"code":"invalid_provider_response","message":"provider returned an invalid response"}`)
		_ = o.Store.Complete(ctx, options.Owner, responseID, TerminalUpdate{Status: StatusFailed, Error: failure})
		return nil, 0, fmt.Errorf("decode provider Responses response: %w", err)
	}
	upstreamID := result.ID
	if upstreamID == "" {
		failure := json.RawMessage(`{"code":"invalid_provider_response","message":"provider response omitted id"}`)
		_ = o.Store.Complete(ctx, options.Owner, responseID, TerminalUpdate{Status: StatusFailed, Error: failure})
		return nil, 0, errors.New("provider Responses response omitted id")
	}
	if err := o.Store.SetProviderCorrelationID(ctx, options.Owner, responseID, upstreamID); err != nil {
		return nil, 0, err
	}
	result.ID = responseID
	result.PreviousResponseID = previous
	outputItems, err := persistenceOutputItems(responseID, result.Output)
	if err != nil {
		return nil, 0, err
	}
	if err := encryptReasoningItems(o.Store, outputItems); err != nil {
		return nil, 0, err
	}
	usage, _ := json.Marshal(result.Usage)
	failure, _ := json.Marshal(result.Error)
	incomplete, _ := json.Marshal(result.IncompleteDetails)
	if err := o.Store.Complete(ctx, options.Owner, responseID, TerminalUpdate{Status: result.Status, Items: outputItems,
		Usage: nullableJSON(result.Usage, usage), Error: nullableJSON(result.Error, failure),
		IncompleteDetails: nullableJSON(result.IncompleteDetails, incomplete)}); err != nil {
		return nil, 0, err
	}
	return &result, providerResponse.StatusCode, nil
}

type UpstreamError struct {
	StatusCode int
	Body       []byte
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("upstream Responses request failed with status %d", e.StatusCode)
}

func applyDefaults(request *CreateRequest) {
	if request.Truncation == "" {
		request.Truncation = TruncationDisabled
	}
	if request.Store == nil {
		value := true
		request.Store = &value
	}
	if request.ParallelToolCalls == nil {
		value := true
		request.ParallelToolCalls = &value
	}
}

func persistenceInputItems(responseID string, input Input) ([]Item, error) {
	if input.String != nil {
		payload, _ := json.Marshal(InputItem{Type: "message", Role: "user", Content: json.RawMessage(mustJSON(*input.String))})
		id, err := (IDGenerator{}).NewItemID()
		if err != nil {
			return nil, err
		}
		return []Item{{ResponseID: responseID, Ordinal: 0, Direction: "input", ItemID: id, ItemType: "message", TokenCount: EstimateTokens(payload), Payload: payload}}, nil
	}
	items := make([]Item, 0, len(input.Items))
	for ordinal, inputItem := range input.Items {
		payload, err := json.Marshal(inputItem)
		if err != nil {
			return nil, err
		}
		id := inputItem.ID
		if id == "" {
			id, err = (IDGenerator{}).NewItemID()
			if err != nil {
				return nil, err
			}
		}
		var callID *string
		if inputItem.CallID != "" {
			value := inputItem.CallID
			callID = &value
		}
		items = append(items, Item{ResponseID: responseID, Ordinal: ordinal, Direction: "input", ItemID: id,
			ItemType: defaultItemType(inputItem.Type), Status: string(inputItem.Status), CallID: callID,
			TokenCount: EstimateTokens(payload), Payload: payload})
	}
	return items, nil
}

func persistenceOutputItems(responseID string, output []OutputItem) ([]Item, error) {
	items := make([]Item, 0, len(output))
	for ordinal, outputItem := range output {
		payload, err := json.Marshal(outputItem)
		if err != nil {
			return nil, err
		}
		var callID *string
		if outputItem.CallID != "" {
			value := outputItem.CallID
			callID = &value
		}
		items = append(items, Item{ResponseID: responseID, Ordinal: ordinal, Direction: "output", ItemID: outputItem.ID,
			ItemType: outputItem.Type, Status: string(outputItem.Status), CallID: callID,
			TokenCount: EstimateTokens(payload), Payload: payload})
	}
	return items, nil
}

func encryptReasoningItems(store StateStore, items []Item) error {
	for i := range items {
		if items[i].ItemType != "reasoning" {
			continue
		}
		var value map[string]any
		if err := json.Unmarshal(items[i].Payload, &value); err != nil {
			return err
		}
		encrypted, _ := value["encrypted_content"].(string)
		if encrypted == "" {
			continue
		}
		delete(value, "encrypted_content")
		payload, err := json.Marshal(value)
		if err != nil {
			return err
		}
		sealed, err := store.EncryptOpaquePayload([]byte(encrypted))
		if err != nil {
			return err
		}
		items[i].Payload, items[i].EncryptedPayload = payload, sealed
	}
	return nil
}

func defaultItemType(value string) string {
	if value == "" {
		return "message"
	}
	return value
}
func mustJSON(value string) []byte { encoded, _ := json.Marshal(value); return encoded }
func nullableJSON(_ any, encoded []byte) json.RawMessage {
	if string(encoded) == "null" {
		return nil
	}
	return encoded
}

// EstimateTokens is a conservative fallback for providers without a model
// tokenizer. It intentionally rounds UTF-8 bytes up at three bytes per token.
func EstimateTokens(payload []byte) int {
	if len(payload) == 0 {
		return 0
	}
	characters := utf8.RuneCount(payload)
	bytesEstimate := (len(payload) + 2) / 3
	if characters > bytesEstimate {
		return characters
	}
	return bytesEstimate
}
