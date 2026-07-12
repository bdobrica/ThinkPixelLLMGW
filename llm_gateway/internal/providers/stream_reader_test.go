package providers

import (
	"io"
	"strings"
	"testing"
)

type chunkedReadCloser struct {
	chunks []string
}

func (r *chunkedReadCloser) Read(p []byte) (int, error) {
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}
	chunk := r.chunks[0]
	r.chunks = r.chunks[1:]
	return copy(p, chunk), nil
}

func (r *chunkedReadCloser) Close() error { return nil }

func TestStreamReaderIgnoresNetworkReadBoundaries(t *testing.T) {
	reader := NewStreamReader(&chunkedReadCloser{chunks: []string{
		"data: {\"id\":\"one\",\"cho",
		"ices\":[]}\n\ndata: {\"usage\":{\"input_tokens\":2}}\n\n",
		"data: [DONE]\n\n",
	}})

	first, err := reader.Read()
	if err != nil || string(first.Data) != `{"id":"one","choices":[]}` {
		t.Fatalf("first event = %q, %v", first.Data, err)
	}
	second, err := reader.Read()
	if err != nil || !strings.Contains(string(second.Data), `"input_tokens":2`) {
		t.Fatalf("second event = %q, %v", second.Data, err)
	}
	if event, err := reader.Read(); err != io.EOF || !event.Done {
		t.Fatalf("DONE event = %#v, %v", event, err)
	}
}

func TestStreamReaderSupportsSSEFieldsCommentsAndMultilineData(t *testing.T) {
	stream := io.NopCloser(strings.NewReader(": ping\nevent: message\ndata: {\"value\":\ndata: 1}\r\n\r\n"))
	event, err := NewStreamReader(stream).Read()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(event.Data), "{\"value\":\n1}"; got != want {
		t.Fatalf("data = %q, want %q", got, want)
	}
}
