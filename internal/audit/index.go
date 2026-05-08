package audit

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
)

type Indexer struct {
	repo *repos.AuditIndex
	ch   chan IndexedRecord
	wg   sync.WaitGroup

	mu  sync.Mutex
	err error
}

func NewIndexer(db *sql.DB) *Indexer {
	i := &Indexer{
		repo: repos.NewAuditIndex(db),
		ch:   make(chan IndexedRecord, WriterBuffer),
	}
	i.wg.Add(1)
	go i.run()
	return i
}

func (i *Indexer) Enqueue(ctx context.Context, indexed IndexedRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case i.ch <- indexed:
		return nil
	}
}

func (i *Indexer) Close() error {
	close(i.ch)
	i.wg.Wait()
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.err
}

func (i *Indexer) run() {
	defer i.wg.Done()
	for indexed := range i.ch {
		if err := i.repo.Put(context.Background(), repos.AuditEntry{
			RequestID:   indexed.Record.RequestID,
			TS:          indexed.Record.TS,
			BridgeKeyID: indexed.Record.BridgeKeyID,
			PoolName:    indexed.Record.ModelAlias,
			UpstreamID:  indexed.Record.UpstreamID,
			Status:      indexed.Record.Status,
			CostCents:   int64(indexed.Record.CostCents),
			FilePath:    indexed.FilePath,
			FileOffset:  indexed.FileOffset,
			FileLength:  indexed.FileLength,
		}); err != nil {
			i.setErr(fmt.Errorf("insert audit index %q: %w", indexed.Record.RequestID, err))
		}
	}
}

func (i *Indexer) setErr(err error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.err == nil {
		i.err = err
	}
}
