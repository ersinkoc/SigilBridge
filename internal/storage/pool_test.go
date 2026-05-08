package storage

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

func TestPoolConcurrentWriteRead(t *testing.T) {
	ctx := context.Background()
	pool, err := OpenPool(filepath.Join(t.TempDir(), "pool.db"))
	if err != nil {
		t.Fatalf("OpenPool() error = %v", err)
	}
	defer pool.Close()

	if _, err := pool.ExecW(ctx, "CREATE TABLE items (id INTEGER PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := pool.ExecW(ctx, "INSERT INTO items (id, value) VALUES (?, ?)", i, "ok"); err != nil {
				errs <- err
				return
			}
			rows, err := pool.QueryR(ctx, "SELECT value FROM items WHERE id = ?", i)
			if err != nil {
				errs <- err
				return
			}
			defer rows.Close()
			if !rows.Next() {
				errs <- rows.Err()
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent operation error = %v", err)
		}
	}
}
