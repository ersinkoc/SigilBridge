package cliacp

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
)

func TestCodecRoundTrip(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	client := NewCodec(left)
	server := NewCodec(right)
	go func() {
		_ = client.Send(Message{ID: 1, Method: MethodInitialize, Params: json.RawMessage(`{"client_name":"test","client_version":"dev"}`)})
	}()
	msg, err := server.Recv()
	if err != nil {
		t.Fatalf("Recv() error = %v", err)
	}
	if msg.Method != MethodInitialize {
		t.Fatalf("message = %#v", msg)
	}
}

func TestNDJSONCodecRoundTrip(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	client := NewNDJSONCodec(left)
	server := NewNDJSONCodec(right)
	go func() {
		_ = client.Send(Message{ID: "1", Method: MethodSessionPrompt, Params: json.RawMessage(`{"sessionId":"s1","prompt":[{"type":"text","text":"hi"}]}`)})
	}()
	msg, err := server.Recv()
	if err != nil {
		t.Fatalf("Recv() error = %v", err)
	}
	if msg.Method != MethodSessionPrompt || msg.ID != "1" {
		t.Fatalf("message = %#v", msg)
	}
}

func TestCodecMalformedFrame(t *testing.T) {
	rwc := readWriteCloser{Reader: strings.NewReader("Bad\r\n\r\n{}"), Writer: discardWriter{}, Closer: noopCloser{}}
	_, err := NewCodec(rwc).Recv()
	if err == nil {
		t.Fatalf("Recv() expected error")
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

type noopCloser struct{}

func (noopCloser) Close() error { return nil }
