package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func startTestServer(t *testing.T, server *http.Server) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve(listener) }()
	return "http://" + listener.Addr().String(), func() {
		_ = server.Close()
		_ = listener.Close()
	}
}

func TestStreamingResponseClearsServerWriteDeadline(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
			t.Errorf("clear write deadline: %v", err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(150 * time.Millisecond) // Three times the non-streaming deadline.
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})
	server := &http.Server{Handler: handler, WriteTimeout: 50 * time.Millisecond, ReadHeaderTimeout: time.Second}
	baseURL, closeServer := startTestServer(t, server)
	defer closeServer()

	response, err := http.Get(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || !strings.Contains(string(body), "[DONE]") {
		t.Fatalf("long stream body=%q err=%v", body, err)
	}
}

func TestNonStreamingResponseRemainsWriteBounded(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = io.WriteString(w, "late response")
	})
	server := &http.Server{Handler: handler, WriteTimeout: 50 * time.Millisecond, ReadHeaderTimeout: time.Second}
	baseURL, closeServer := startTestServer(t, server)
	defer closeServer()
	client := &http.Client{Timeout: time.Second}

	response, err := client.Get(baseURL)
	if err == nil {
		_, err = io.ReadAll(response.Body)
		_ = response.Body.Close()
	}
	if err == nil {
		t.Fatal("expected the non-streaming write deadline to terminate the response")
	}
}

func TestSlowHeadersAreBounded(t *testing.T) {
	server := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: 50 * time.Millisecond}
	baseURL, closeServer := startTestServer(t, server)
	defer closeServer()
	address := strings.TrimPrefix(baseURL, "http://")
	connection, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_, _ = fmt.Fprintf(connection, "GET / HTTP/1.1\r\nHost: test")
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	buffer := make([]byte, 1)
	if _, err := connection.Read(buffer); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("expected server close/response before client deadline, got %v", err)
	}
}

func TestShutdownDeadlineCanForceCloseActiveStream(t *testing.T) {
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}
	baseURL, closeServer := startTestServer(t, server)
	defer closeServer()
	go func() { _, _ = http.Get(baseURL) }()
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := server.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown error = %v, want deadline exceeded", err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
}
