package providers

import (
	"context"
	"testing"
	"time"
)

type testProvider struct {
	id       string
	closedCh chan struct{}
}

func newTestProvider(id string) *testProvider {
	return &testProvider{id: id, closedCh: make(chan struct{})}
}

func (p *testProvider) ID() string   { return p.id }
func (p *testProvider) Name() string { return p.id }
func (p *testProvider) Type() string { return "test" }
func (p *testProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return nil, nil
}
func (p *testProvider) ValidateCredentials(ctx context.Context) error { return nil }
func (p *testProvider) Close() error {
	select {
	case <-p.closedCh:
	default:
		close(p.closedCh)
	}
	return nil
}

func TestProviderRegistryRetireProviders_DelaysClosure(t *testing.T) {
	rawProvider := newTestProvider("provider-1")
	provider := newManagedProvider(rawProvider)
	registry := &ProviderRegistry{
		closeGracePeriod: 40 * time.Millisecond,
		stopCh:           make(chan struct{}),
	}

	registry.retireProviders([]*managedProvider{provider})

	select {
	case <-rawProvider.closedCh:
		t.Fatal("provider closed immediately; expected graceful delay")
	case <-time.After(15 * time.Millisecond):
	}

	select {
	case <-rawProvider.closedCh:
	case <-time.After(120 * time.Millisecond):
		t.Fatal("provider was not closed after grace period")
	}

	close(registry.stopCh)
	registry.wg.Wait()
}

func TestProviderRegistryRetireProviders_StopClosesEarly(t *testing.T) {
	rawProvider := newTestProvider("provider-2")
	provider := newManagedProvider(rawProvider)
	registry := &ProviderRegistry{
		closeGracePeriod: time.Minute,
		stopCh:           make(chan struct{}),
	}

	registry.retireProviders([]*managedProvider{provider})
	close(registry.stopCh)

	select {
	case <-rawProvider.closedCh:
	case <-time.After(120 * time.Millisecond):
		t.Fatal("provider was not closed promptly after registry stop")
	}

	registry.wg.Wait()
}

func TestManagedProvider_CloseIsIdempotent(t *testing.T) {
	rawProvider := newTestProvider("provider-3")
	provider := newManagedProvider(rawProvider)

	if err := provider.Close(); err != nil {
		t.Fatalf("first close returned error: %v", err)
	}
	if err := provider.Close(); err != nil {
		t.Fatalf("second close returned error: %v", err)
	}

	select {
	case <-rawProvider.closedCh:
	default:
		t.Fatal("provider should have been closed")
	}
}
