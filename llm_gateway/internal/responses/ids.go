package responses

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// IDGenerator generates independently scoped, collision-resistant public IDs.
type IDGenerator struct {
	Reader io.Reader
}

func (g IDGenerator) NewResponseID() (string, error) { return g.newID("resp_", 24) }
func (g IDGenerator) NewItemID() (string, error)     { return g.newID("item_", 24) }
func (g IDGenerator) NewCallID() (string, error)     { return g.newID("call_", 24) }
func (g IDGenerator) NewToolExecutionID() (string, error) {
	return g.newID("toolx_", 24)
}

func (g IDGenerator) newID(prefix string, bytes int) (string, error) {
	reader := g.Reader
	if reader == nil {
		reader = rand.Reader
	}
	value := make([]byte, bytes)
	if _, err := io.ReadFull(reader, value); err != nil {
		return "", fmt.Errorf("generate %s ID: %w", prefix, err)
	}
	return prefix + hex.EncodeToString(value), nil
}
