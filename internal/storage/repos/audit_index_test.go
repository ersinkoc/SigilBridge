package repos

import (
	"context"
	"testing"
	"time"
)

func TestAuditIndexCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewAuditIndex(newTestDB(t))
	entry := AuditEntry{RequestID: "01HXREQ", TS: testTime(), Status: "success", FilePath: "audit/2026-05-07.jsonl", FileOffset: 1, FileLength: 2}
	if err := repo.Put(ctx, entry); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, err := repo.Get(ctx, entry.RequestID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.FilePath != entry.FilePath {
		t.Fatalf("FilePath = %q, want %q", got.FilePath, entry.FilePath)
	}
	list, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() len = %d, want 1", len(list))
	}
	if err := repo.Delete(ctx, entry.RequestID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestAuditIndexQueryFiltersAndCursor(t *testing.T) {
	ctx := context.Background()
	repo := NewAuditIndex(newTestDB(t))
	base := testTime()
	entries := []AuditEntry{
		{RequestID: "req_4", TS: base.Add(4 * time.Minute), BridgeKeyID: "key1", PoolName: "main", UpstreamID: "upstream-a", Status: "ok", FilePath: "audit/4.jsonl"},
		{RequestID: "req_3", TS: base.Add(3 * time.Minute), BridgeKeyID: "key2", PoolName: "main", UpstreamID: "upstream-b", Status: "error", FilePath: "audit/3.jsonl"},
		{RequestID: "req_2", TS: base.Add(2 * time.Minute), BridgeKeyID: "key1", PoolName: "backup", UpstreamID: "upstream-a", Status: "ok", FilePath: "audit/2.jsonl"},
		{RequestID: "req_1", TS: base.Add(1 * time.Minute), BridgeKeyID: "key1", PoolName: "main", UpstreamID: "upstream-c", Status: "ok", FilePath: "audit/1.jsonl"},
	}
	for _, entry := range entries {
		if err := repo.Put(ctx, entry); err != nil {
			t.Fatalf("Put(%s) error = %v", entry.RequestID, err)
		}
	}

	filtered, err := repo.Query(ctx, AuditQuery{BridgeKeyID: "key1", PoolName: "main", Status: "ok", Limit: 10})
	if err != nil {
		t.Fatalf("Query(filter) error = %v", err)
	}
	if got, want := requestIDs(filtered.Entries), []string{"req_4", "req_1"}; !sameStrings(got, want) {
		t.Fatalf("filtered request IDs = %#v, want %#v", got, want)
	}

	byRequest, err := repo.Query(ctx, AuditQuery{RequestID: "req_3", Limit: 10})
	if err != nil {
		t.Fatalf("Query(request_id) error = %v", err)
	}
	if got, want := requestIDs(byRequest.Entries), []string{"req_3"}; !sameStrings(got, want) {
		t.Fatalf("request_id request IDs = %#v, want %#v", got, want)
	}

	byUpstream, err := repo.Query(ctx, AuditQuery{UpstreamID: "upstream-a", Limit: 10})
	if err != nil {
		t.Fatalf("Query(upstream_id) error = %v", err)
	}
	if got, want := requestIDs(byUpstream.Entries), []string{"req_4", "req_2"}; !sameStrings(got, want) {
		t.Fatalf("upstream_id request IDs = %#v, want %#v", got, want)
	}

	page1, err := repo.Query(ctx, AuditQuery{Limit: 2})
	if err != nil {
		t.Fatalf("Query(page1) error = %v", err)
	}
	if page1.NextCursor == "" {
		t.Fatalf("NextCursor is empty")
	}
	if got, want := requestIDs(page1.Entries), []string{"req_4", "req_3"}; !sameStrings(got, want) {
		t.Fatalf("page1 request IDs = %#v, want %#v", got, want)
	}
	page2, err := repo.Query(ctx, AuditQuery{Limit: 2, Cursor: page1.NextCursor})
	if err != nil {
		t.Fatalf("Query(page2) error = %v", err)
	}
	if got, want := requestIDs(page2.Entries), []string{"req_2", "req_1"}; !sameStrings(got, want) {
		t.Fatalf("page2 request IDs = %#v, want %#v", got, want)
	}
}

func requestIDs(entries []AuditEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.RequestID)
	}
	return out
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
