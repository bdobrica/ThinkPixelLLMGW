package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func TestModel_CalculateCost(t *testing.T) {
	model := &Model{
		ID:        uuid.New(),
		ModelName: "gpt-4",
	}

	// Create a usage record
	usageRecord := UsageRecord{
		InputTokens:  100,
		OutputTokens: 50,
	}

	cost := model.CalculateCost(usageRecord)
	if cost != 0.0 {
		t.Errorf("CalculateCost() = %f, want 0.0 (not implemented yet)", cost)
	}
}

func TestModel_SupportedFeatures(t *testing.T) {
	model := &Model{
		ID:                              uuid.New(),
		ModelName:                       "advanced-model",
		SupportsFunctionCalling:         true,
		SupportsVision:                  true,
		SupportsStreamingOutput:         true,
		SupportsResponseSchema:          true,
		SupportsParallelFunctionCalling: true,
	}

	if !model.SupportsFunctionCalling {
		t.Error("Model should support function calling")
	}
	if !model.SupportsVision {
		t.Error("Model should support vision")
	}
	if !model.SupportsStreamingOutput {
		t.Error("Model should support streaming output")
	}
	if !model.SupportsResponseSchema {
		t.Error("Model should support response schema")
	}
}

func TestModel_Limits(t *testing.T) {
	model := &Model{
		ID:                 uuid.New(),
		ModelName:          "test-model",
		MaxTokens:          128000,
		MaxInputTokens:     100000,
		MaxOutputTokens:    4096,
		TokensPerMinute:    90000,
		RequestsPerMinute:  3000,
		MaxImagesPerPrompt: 10,
	}

	if model.MaxTokens != 128000 {
		t.Errorf("MaxTokens = %d, want 128000", model.MaxTokens)
	}
	if model.MaxInputTokens != 100000 {
		t.Errorf("MaxInputTokens = %d, want 100000", model.MaxInputTokens)
	}
	if model.MaxOutputTokens != 4096 {
		t.Errorf("MaxOutputTokens = %d, want 4096", model.MaxOutputTokens)
	}
	if model.TokensPerMinute != 90000 {
		t.Errorf("TokensPerMinute = %d, want 90000", model.TokensPerMinute)
	}
	if model.RequestsPerMinute != 3000 {
		t.Errorf("RequestsPerMinute = %d, want 3000", model.RequestsPerMinute)
	}
}

func TestModel_Deprecation(t *testing.T) {
	now := time.Now()
	future := now.Add(30 * 24 * time.Hour)

	t.Run("not deprecated", func(t *testing.T) {
		model := &Model{
			ID:           uuid.New(),
			ModelName:    "current-model",
			IsDeprecated: false,
		}
		if model.IsDeprecated {
			t.Error("Model should not be deprecated")
		}
	})

	t.Run("deprecated with date", func(t *testing.T) {
		model := &Model{
			ID:              uuid.New(),
			ModelName:       "old-model",
			IsDeprecated:    true,
			DeprecationDate: &future,
		}
		if !model.IsDeprecated {
			t.Error("Model should be deprecated")
		}
		if model.DeprecationDate == nil {
			t.Error("DeprecationDate should not be nil")
		}
	})
}

func TestModel_Regions(t *testing.T) {
	model := &Model{
		ID:               uuid.New(),
		ModelName:        "global-model",
		SupportedRegions: pq.StringArray{"us-east-1", "eu-west-1", "ap-southeast-1"},
	}

	if len(model.SupportedRegions) != 3 {
		t.Errorf("Expected 3 supported regions, got %d", len(model.SupportedRegions))
	}

	expectedRegions := map[string]bool{
		"us-east-1":      true,
		"eu-west-1":      true,
		"ap-southeast-1": true,
	}

	for _, region := range model.SupportedRegions {
		if !expectedRegions[region] {
			t.Errorf("Unexpected region: %s", region)
		}
	}
}

func TestModel_PricingMetadata(t *testing.T) {
	schemaVersion := "v1"
	slaVersion := "gold"
	metadataVersion := "1.0"

	model := &Model{
		ID:                            uuid.New(),
		ModelName:                     "pricing-model",
		Currency:                      "USD",
		PricingComponentSchemaVersion: &schemaVersion,
		SLATier:                       &slaVersion,
		SupportsSLA:                   true,
		MetadataSchemaVersion:         &metadataVersion,
		AvailabilitySLO:               99.9,
		AverageLatencyMs:              250.5,
		P95LatencyMs:                  500.0,
	}

	if model.Currency != "USD" {
		t.Errorf("Currency = %s, want USD", model.Currency)
	}
	if model.PricingComponentSchemaVersion == nil || *model.PricingComponentSchemaVersion != "v1" {
		t.Error("PricingComponentSchemaVersion should be v1")
	}
	if !model.SupportsSLA {
		t.Error("Model should support SLA")
	}
	if model.AvailabilitySLO != 99.9 {
		t.Errorf("AvailabilitySLO = %f, want 99.9", model.AvailabilitySLO)
	}
}

func TestModel_MultimodalSupport(t *testing.T) {
	model := &Model{
		ID:                  uuid.New(),
		ModelName:           "multimodal-model",
		SupportsVision:      true,
		SupportsAudioInput:  true,
		SupportsAudioOutput: true,
		SupportsVideoInput:  true,
		SupportsPDFInput:    true,
		MaxImagesPerPrompt:  20,
		MaxAudioPerPrompt:   5,
		MaxVideosPerPrompt:  3,
		MaxPDFSizeMB:        100,
	}

	if !model.SupportsVision {
		t.Error("Model should support vision")
	}
	if !model.SupportsAudioInput {
		t.Error("Model should support audio input")
	}
	if !model.SupportsVideoInput {
		t.Error("Model should support video input")
	}
	if model.MaxImagesPerPrompt != 20 {
		t.Errorf("MaxImagesPerPrompt = %d, want 20", model.MaxImagesPerPrompt)
	}
	if model.MaxPDFSizeMB != 100 {
		t.Errorf("MaxPDFSizeMB = %d, want 100", model.MaxPDFSizeMB)
	}
}

func TestModel_PricingComponents(t *testing.T) {
	modelID := uuid.New().String()

	model := &Model{
		ID:        uuid.New(),
		ModelName: "test-model",
		PricingComponents: []PricingComponent{
			{
				ID:        uuid.New().String(),
				ModelID:   modelID,
				Code:      "input_text_default",
				Direction: PricingDirectionInput,
				Modality:  PricingModalityText,
				Unit:      PricingUnit1KTokens,
				Price:     0.03,
			},
			{
				ID:        uuid.New().String(),
				ModelID:   modelID,
				Code:      "output_text_default",
				Direction: PricingDirectionOutput,
				Modality:  PricingModalityText,
				Unit:      PricingUnit1KTokens,
				Price:     0.06,
			},
		},
	}

	if len(model.PricingComponents) != 2 {
		t.Errorf("Expected 2 pricing components, got %d", len(model.PricingComponents))
	}

	// Verify input tokens component
	inputComponent := model.PricingComponents[0]
	if inputComponent.Direction != PricingDirectionInput {
		t.Errorf("First component direction = %s, want %s", inputComponent.Direction, PricingDirectionInput)
	}
	if inputComponent.Price != 0.03 {
		t.Errorf("Input price = %f, want 0.03", inputComponent.Price)
	}
	if inputComponent.Unit != PricingUnit1KTokens {
		t.Errorf("Input unit = %s, want %s", inputComponent.Unit, PricingUnit1KTokens)
	}

	// Verify output tokens component
	outputComponent := model.PricingComponents[1]
	if outputComponent.Direction != PricingDirectionOutput {
		t.Errorf("Second component direction = %s, want %s", outputComponent.Direction, PricingDirectionOutput)
	}
	if outputComponent.Price != 0.06 {
		t.Errorf("Output price = %f, want 0.06", outputComponent.Price)
	}
}

func TestModel_Timestamps(t *testing.T) {
	now := time.Now()
	model := &Model{
		ID:        uuid.New(),
		ModelName: "timestamped-model",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if model.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if model.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
	if !model.CreatedAt.Equal(model.UpdatedAt) {
		t.Error("CreatedAt and UpdatedAt should be equal for new model")
	}
}
