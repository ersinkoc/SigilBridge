package events

import (
	"encoding/json"
	"net/http"
	"sync"
)

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type Bus struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

func NewBus() *Bus {
	return &Bus{subs: map[chan Event]struct{}{}}
}

func (b *Bus) Publish(event Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *Bus) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 16)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		close(ch)
		b.mu.Unlock()
	}
}

func (b *Bus) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	ch, cancel := b.Subscribe()
	defer cancel()
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	for {
		select {
		case event := <-ch:
			raw, _ := json.Marshal(event.Data)
			_, _ = w.Write([]byte("event: " + event.Type + "\n"))
			_, _ = w.Write([]byte("data: " + string(raw) + "\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}
