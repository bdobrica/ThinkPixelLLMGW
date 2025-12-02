package models

import (
	"testing"

	"github.com/google/uuid"
)

// TestModelCalculateCost tests the cost calculation functionality
func TestModelCalculateCost(t *testing.T) {
	tests := []struct {
		name              string
		pricingComponents []PricingComponent
		usageRecord       UsageRecord
		expectedCost      float64
		description       string
	}{
		{
			name: "OpenAI GPT-4o - input and output tokens",
			pricingComponents: []PricingComponent{
				{
					ID:        uuid.New().String(),
					ModelID:   uuid.New().String(),
					Code:      "input_text_default",
					Direction: PricingDirectionInput,
					Modality:  PricingModalityText,
					Unit:      PricingUnit1KTokens,
					Price:     0.0025, // $2.50 per 1M tokens = $0.0025 per 1K tokens
				},
				{
					ID:        uuid.New().String(),
					ModelID:   uuid.New().String(),
					Code:      "output_text_default",
					Direction: PricingDirectionOutput,
					Modality:  PricingModalityText,
					Unit:      PricingUnit1KTokens,
					Price:     0.01, // $10.00 per 1M tokens = $0.01 per 1K tokens
				},
			},
			usageRecord: UsageRecord{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			expectedCost: 0.0075, // (1000/1000 * 0.0025) + (500/1000 * 0.01) = 0.0025 + 0.005 = 0.0075
			description:  "Standard GPT-4o pricing with 1000 input and 500 output tokens",
		},
		{
			name: "OpenAI GPT-4o - with cached tokens",
			pricingComponents: []PricingComponent{
				{
					ID:        uuid.New().String(),
					ModelID:   uuid.New().String(),
					Code:      "input_text_default",
					Direction: PricingDirectionInput,
					Modality:  PricingModalityText,
					Unit:      PricingUnit1KTokens,
					Price:     0.0025,
				},
				{
					ID:        uuid.New().String(),
					ModelID:   uuid.New().String(),
					Code:      "output_text_default",
					Direction: PricingDirectionOutput,
					Modality:  PricingModalityText,
					Unit:      PricingUnit1KTokens,
					Price:     0.01,
				},
				{
					ID:        uuid.New().String(),
					ModelID:   uuid.New().String(),
					Code:      "cache_text_default",
					Direction: PricingDirectionCache,
					Modality:  PricingModalityText,
					Unit:      PricingUnit1KTokens,
					Price:     0.00125, // Cached tokens typically cost 50% of input tokens
				},
			},
			usageRecord: UsageRecord{
				InputTokens:  500, // Regular input tokens
				OutputTokens: 300, // Output tokens
				CachedTokens: 500, // Cached tokens (separate from input)
			},
			expectedCost: 0.004875, // (500/1000 * 0.0025) + (300/1000 * 0.01) + (500/1000 * 0.00125) = 0.00125 + 0.003 + 0.000625
			description:  "With prompt caching - cached tokens priced separately",
		},
		{
			name: "OpenAI o1 - with reasoning tokens",
			pricingComponents: []PricingComponent{
				{
					ID:        uuid.New().String(),
					ModelID:   uuid.New().String(),
					Code:      "input_text_default",
					Direction: PricingDirectionInput,
					Modality:  PricingModalityText,
					Unit:      PricingUnit1KTokens,
					Price:     0.015, // o1 is more expensive
				},
				{
					ID:        uuid.New().String(),
					ModelID:   uuid.New().String(),
					Code:      "output_text_default",
					Direction: PricingDirectionOutput,
					Modality:  PricingModalityText,
					Unit:      PricingUnit1KTokens,
					Price:     0.06, // o1 output is very expensive
				},
			},
			usageRecord: UsageRecord{
				InputTokens:     200,
				OutputTokens:    100,  // Regular output tokens
				ReasoningTokens: 1000, // Reasoning tokens (separate from output, but priced same)
			},
			expectedCost: 0.069, // (200/1000 * 0.015) + (100/1000 * 0.06) + (1000/1000 * 0.06) = 0.003 + 0.006 + 0.06 = 0.069
			description:  "o1 model with reasoning tokens (priced as output tokens, counted separately)",
		},
		{
			name: "Per-token pricing (alternative unit)",
			pricingComponents: []PricingComponent{
				{
					ID:        uuid.New().String(),
					ModelID:   uuid.New().String(),
					Code:      "input_text_default",
					Direction: PricingDirectionInput,
					Modality:  PricingModalityText,
					Unit:      PricingUnitToken, // Price per single token
					Price:     0.0000025,        // $2.50 per 1M tokens
				},
				{
					ID:        uuid.New().String(),
					ModelID:   uuid.New().String(),
					Code:      "output_text_default",
					Direction: PricingDirectionOutput,
					Modality:  PricingModalityText,
					Unit:      PricingUnitToken,
					Price:     0.00001, // $10.00 per 1M tokens
				},
			},
			usageRecord: UsageRecord{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			expectedCost: 0.0075, // (1000 * 0.0000025) + (500 * 0.00001) = 0.0025 + 0.005
			description:  "Per-token pricing instead of per-1K-tokens",
		},
		{
			name: "Zero tokens - no cost",
			pricingComponents: []PricingComponent{
				{
					ID:        uuid.New().String(),
					ModelID:   uuid.New().String(),
					Code:      "input_text_default",
					Direction: PricingDirectionInput,
					Modality:  PricingModalityText,
					Unit:      PricingUnit1KTokens,
					Price:     0.0025,
				},
				{
					ID:        uuid.New().String(),
					ModelID:   uuid.New().String(),
					Code:      "output_text_default",
					Direction: PricingDirectionOutput,
					Modality:  PricingModalityText,
					Unit:      PricingUnit1KTokens,
					Price:     0.01,
				},
			},
			usageRecord: UsageRecord{
				InputTokens:  0,
				OutputTokens: 0,
			},
			expectedCost: 0.0,
			description:  "No tokens used should result in zero cost",
		},
		{
			name:              "No pricing components",
			pricingComponents: []PricingComponent{},
			usageRecord: UsageRecord{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			expectedCost: 0.0,
			description:  "Missing pricing components should result in zero cost (fallback)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &Model{
				ID:                uuid.New(),
				ModelName:         "test-model",
				Currency:          "USD",
				PricingComponents: tt.pricingComponents,
			}

			actualCost := model.CalculateCost(tt.usageRecord)

			// Allow for floating point precision differences
			tolerance := 0.0001
			if actualCost < tt.expectedCost-tolerance || actualCost > tt.expectedCost+tolerance {
				t.Errorf("%s: expected cost %.6f, got %.6f (difference: %.6f)",
					tt.description, tt.expectedCost, actualCost, actualCost-tt.expectedCost)
			} else {
				t.Logf("%s: ✓ Cost calculated correctly: $%.6f", tt.description, actualCost)
			}
		})
	}
}

// TestFindPricingComponent tests the pricing component lookup logic
func TestFindPricingComponent(t *testing.T) {
	defaultTier := string(PricingTierDefault)
	premiumTier := string(PricingTierPremium)

	components := []PricingComponent{
		{
			ID:        "1",
			Direction: PricingDirectionInput,
			Modality:  PricingModalityText,
			Unit:      PricingUnit1KTokens,
			Tier:      &defaultTier,
			Price:     0.001,
		},
		{
			ID:        "2",
			Direction: PricingDirectionInput,
			Modality:  PricingModalityText,
			Unit:      PricingUnit1KTokens,
			Tier:      &premiumTier,
			Price:     0.002,
		},
		{
			ID:        "3",
			Direction: PricingDirectionOutput,
			Modality:  PricingModalityText,
			Unit:      PricingUnit1KTokens,
			Tier:      nil, // No tier = default
			Price:     0.003,
		},
	}

	model := &Model{
		ID:                uuid.New(),
		ModelName:         "test-model",
		PricingComponents: components,
	}

	// Test finding input component - should prefer default tier
	inputComponent := model.findPricingComponent(PricingDirectionInput, PricingModalityText)
	if inputComponent == nil {
		t.Fatal("Expected to find input component")
	}
	if inputComponent.Price != 0.001 {
		t.Errorf("Expected to find default tier component with price 0.001, got %.3f", inputComponent.Price)
	}

	// Test finding output component - should find the one with nil tier
	outputComponent := model.findPricingComponent(PricingDirectionOutput, PricingModalityText)
	if outputComponent == nil {
		t.Fatal("Expected to find output component")
	}
	if outputComponent.Price != 0.003 {
		t.Errorf("Expected to find component with price 0.003, got %.3f", outputComponent.Price)
	}

	// Test finding non-existent component
	cacheComponent := model.findPricingComponent(PricingDirectionCache, PricingModalityText)
	if cacheComponent != nil {
		t.Error("Expected nil for non-existent cache component")
	}
}

// TestCalculateComponentCost tests the per-component cost calculation
func TestCalculateComponentCost(t *testing.T) {
	model := &Model{}

	tests := []struct {
		name         string
		component    *PricingComponent
		tokens       int
		expectedCost float64
	}{
		{
			name: "1K tokens unit",
			component: &PricingComponent{
				Unit:  PricingUnit1KTokens,
				Price: 0.01,
			},
			tokens:       500,
			expectedCost: 0.005, // 500/1000 * 0.01
		},
		{
			name: "Per token unit",
			component: &PricingComponent{
				Unit:  PricingUnitToken,
				Price: 0.00001,
			},
			tokens:       500,
			expectedCost: 0.005, // 500 * 0.00001
		},
		{
			name:         "Nil component",
			component:    nil,
			tokens:       500,
			expectedCost: 0.0,
		},
		{
			name: "Zero tokens",
			component: &PricingComponent{
				Unit:  PricingUnit1KTokens,
				Price: 0.01,
			},
			tokens:       0,
			expectedCost: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := model.calculateComponentCost(tt.component, tt.tokens)
			if cost != tt.expectedCost {
				t.Errorf("Expected cost %.6f, got %.6f", tt.expectedCost, cost)
			}
		})
	}
}
