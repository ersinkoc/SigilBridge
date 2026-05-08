package storage

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"sync"
)

type Pool struct {
	writer *sql.DB
	reader *sql.DB
	mu     sync.Mutex
}

func OpenPool(path string) (*Pool, error) {
	writer, err := OpenDB(path)
	if err != nil {
		return nil, err
	}
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)

	reader, err := OpenDB(path)
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	reader.SetMaxOpenConns(runtime.NumCPU())
	reader.SetMaxIdleConns(runtime.NumCPU())

	return &Pool{writer: writer, reader: reader}, nil
}

func NewPool(writer, reader *sql.DB) (*Pool, error) {
	if writer == nil {
		return nil, fmt.Errorf("writer database is nil")
	}
	if reader == nil {
		reader = writer
	}
	return &Pool{writer: writer, reader: reader}, nil
}

func (p *Pool) ExecW(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if p == nil || p.writer == nil {
		return nil, fmt.Errorf("storage pool writer is nil")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.writer.ExecContext(ctx, query, args...)
}

func (p *Pool) QueryR(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if p == nil || p.reader == nil {
		return nil, fmt.Errorf("storage pool reader is nil")
	}
	return p.reader.QueryContext(ctx, query, args...)
}

func (p *Pool) QueryRowR(ctx context.Context, query string, args ...any) *sql.Row {
	return p.reader.QueryRowContext(ctx, query, args...)
}

func (p *Pool) Close() error {
	if p == nil {
		return nil
	}
	var err error
	if p.reader != nil && p.reader != p.writer {
		err = p.reader.Close()
	}
	if p.writer != nil {
		if closeErr := p.writer.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}
