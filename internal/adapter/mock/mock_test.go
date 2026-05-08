package mock

import (
	"context"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestMockChatDeterministicAndError(t *testing.T) {
	p := New()
	req := ir.Request{ModelAlias: "test", Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hello"}}}}}
	a, err := p.Chat(context.Background(), req, adapter.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.Chat(context.Background(), req, adapter.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Content[0].Text != b.Content[0].Text {
		t.Fatalf("mock response not deterministic")
	}
	if _, err := p.Chat(context.Background(), req, adapter.ProviderConfig{Raw: map[string]any{"error_type": "boom"}}); err == nil {
		t.Fatal("expected injected error")
	}
}

func TestMockStream(t *testing.T) {
	ch, err := New().Stream(context.Background(), ir.Request{ModelAlias: "m"}, adapter.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Fatal("stream emitted no events")
	}
}
