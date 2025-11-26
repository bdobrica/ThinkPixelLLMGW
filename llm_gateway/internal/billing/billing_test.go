package billing

import (
	"context"
	"testing"
)

func TestNoopService_WithinBudget(t *testing.T) {
	service := NewNoopService()
	ctx := context.Background()

	testCases := []string{
		"api-key-1",
		"api-key-2",
		"",
		"invalid-uuid",
	}

	for _, apiKeyID := range testCases {
		t.Run(apiKeyID, func(t *testing.T) {
			result := service.WithinBudget(ctx, apiKeyID)
			if !result {
				t.Errorf("NoopService.WithinBudget() = false, want true")
			}
		})
	}
}

func TestNoopService_AddUsage(t *testing.T) {
	service := NewNoopService()
	ctx := context.Background()

	testCases := []struct {
		name     string
		apiKeyID string
		cost     float64
	}{
		{
			name:     "positive cost",
			apiKeyID: "api-key-1",
			cost:     10.50,
		},
		{
			name:     "zero cost",
			apiKeyID: "api-key-2",
			cost:     0.0,
		},
		{
			name:     "large cost",
			apiKeyID: "api-key-3",
			cost:     1000.99,
		},
		{
			name:     "small cost",
			apiKeyID: "api-key-4",
			cost:     0.001,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.AddUsage(ctx, tc.apiKeyID, tc.cost)
			if err != nil {
				t.Errorf("NoopService.AddUsage() error = %v, want nil", err)
			}
		})
	}
}

func TestNoopService_Multiple(t *testing.T) {
	service := NewNoopService()
	ctx := context.Background()

	// Add multiple usage entries
	apiKeyID := "test-key"
	costs := []float64{1.0, 2.5, 3.75, 10.0}

	for _, cost := range costs {
		if err := service.AddUsage(ctx, apiKeyID, cost); err != nil {
			t.Errorf("NoopService.AddUsage() error = %v", err)
		}
	}

	// Check budget (should always be within budget)
	if !service.WithinBudget(ctx, apiKeyID) {
		t.Error("NoopService.WithinBudget() = false after adding usage, want true")
	}
}

func TestNoopService_Concurrent(t *testing.T) {
	service := NewNoopService()
	ctx := context.Background()

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			apiKeyID := "concurrent-key"
			service.AddUsage(ctx, apiKeyID, 1.0)
			service.WithinBudget(ctx, apiKeyID)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
