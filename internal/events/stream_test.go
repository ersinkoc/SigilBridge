package events

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBusSubscribeAndStream(t *testing.T) {
	bus := NewBus()
	ch, cancel := bus.Subscribe()
	defer cancel()
	bus.Publish(Event{Type: "reload", Data: map[string]any{"ok": true}})
	select {
	case event := <-ch:
		if event.Type != "reload" {
			t.Fatalf("event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	srv := httptest.NewServer(bus)
	defer srv.Close()
	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	bus.Publish(Event{Type: "ping", Data: "pong"})
	body := readUntil(t, resp.Body, `data: "pong"`)
	if !strings.Contains(body, "event: ping") {
		t.Fatalf("body = %s", body)
	}
}

func readUntil(t *testing.T, r interface {
	Read([]byte) (int, error)
}, want string) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(r)
		var body strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				done <- body.String()
				return
			}
			body.WriteString(line)
			if strings.Contains(body.String(), want) {
				done <- body.String()
				return
			}
		}
	}()
	select {
	case body := <-done:
		if !strings.Contains(body, want) {
			t.Fatalf("body = %s", body)
		}
		return body
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for %q", want)
		return ""
	}
}
