package responses

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestIDGeneratorPrefixesAndEntropy(t *testing.T) {
	generator := IDGenerator{}
	tests := []struct {
		prefix   string
		generate func() (string, error)
	}{
		{"resp_", generator.NewResponseID}, {"item_", generator.NewItemID},
		{"call_", generator.NewCallID}, {"toolx_", generator.NewToolExecutionID},
	}
	for _, test := range tests {
		id, err := test.generate()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(id, test.prefix) || len(id) != len(test.prefix)+48 {
			t.Fatalf("unexpected ID %q", id)
		}
	}
}

func TestIDGeneratorConcurrentUniqueness(t *testing.T) {
	const count = 1000
	ids := make(chan string, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := (IDGenerator{}).NewResponseID()
			if err != nil {
				errs <- err
				return
			}
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	seen := make(map[string]struct{}, count)
	for id := range ids {
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate ID %s", id)
		}
		seen[id] = struct{}{}
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }
func TestIDGeneratorEntropyFailure(t *testing.T) {
	if _, err := (IDGenerator{Reader: failingReader{}}).NewResponseID(); err == nil {
		t.Fatal("expected error")
	}
}
