package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const WriterBuffer = 1024

type Record struct {
	TS               time.Time         `json:"ts"`
	RequestID        string            `json:"request_id"`
	BridgeKeyID      string            `json:"bridge_key_id,omitempty"`
	IngressFormat    string            `json:"ingress_format,omitempty"`
	ModelAlias       string            `json:"model_alias,omitempty"`
	UpstreamProvider string            `json:"upstream_provider,omitempty"`
	UpstreamID       string            `json:"upstream_id,omitempty"`
	UpstreamModel    string            `json:"upstream_model,omitempty"`
	InputTokens      int               `json:"input_tokens,omitempty"`
	OutputTokens     int               `json:"output_tokens,omitempty"`
	CacheReadTokens  int               `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int               `json:"cache_write_tokens,omitempty"`
	CostCents        int               `json:"cost_cents,omitempty"`
	LatencyMs        int64             `json:"latency_ms,omitempty"`
	TTFBMs           int64             `json:"ttfb_ms,omitempty"`
	Stream           bool              `json:"stream"`
	StopReason       string            `json:"stop_reason,omitempty"`
	Status           string            `json:"status"`
	Error            *RecordError      `json:"error,omitempty"`
	Retries          int               `json:"retries,omitempty"`
	ClientIPHash     string            `json:"client_ip_hash,omitempty"`
	UserAgent        string            `json:"user_agent,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Content          CapturedContent   `json:"content,omitempty"`
}

type RecordError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type IndexedRecord struct {
	Record     Record
	FilePath   string
	FileOffset int64
	FileLength int64
	Line       []byte
}

type Writer struct {
	dir     string
	indexer *Indexer
	ch      chan Record
	wg      sync.WaitGroup

	mu          sync.Mutex
	err         error
	currentDay  string
	currentFile *os.File
}

func NewWriter(dir string, indexer *Indexer) (*Writer, error) {
	if dir == "" {
		return nil, fmt.Errorf("audit directory is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create audit directory: %w", err)
	}
	w := &Writer{dir: dir, indexer: indexer, ch: make(chan Record, WriterBuffer)}
	w.wg.Add(1)
	go w.run()
	return w, nil
}

func (w *Writer) Write(ctx context.Context, record Record) error {
	if record.TS.IsZero() {
		record.TS = time.Now().UTC()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case w.ch <- record:
		return nil
	}
}

func (w *Writer) Close() error {
	close(w.ch)
	w.wg.Wait()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.currentFile != nil {
		if err := w.currentFile.Sync(); w.err == nil {
			w.err = err
		}
		if err := w.currentFile.Close(); w.err == nil {
			w.err = err
		}
		w.currentFile = nil
	}
	return w.err
}

func (w *Writer) run() {
	defer w.wg.Done()
	for record := range w.ch {
		indexed, err := w.append(record)
		if err != nil {
			w.setErr(err)
			continue
		}
		if w.indexer != nil {
			if err := w.indexer.Enqueue(context.Background(), indexed); err != nil {
				w.setErr(fmt.Errorf("enqueue audit index %q: %w", indexed.Record.RequestID, err))
			}
		}
	}
}

func (w *Writer) append(record Record) (IndexedRecord, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	day := record.TS.UTC().Format("2006-01-02")
	if err := w.openDay(day); err != nil {
		return IndexedRecord{}, err
	}
	offset, err := w.currentFile.Seek(0, 2)
	if err != nil {
		return IndexedRecord{}, fmt.Errorf("seek audit file: %w", err)
	}
	line, err := json.Marshal(record)
	if err != nil {
		return IndexedRecord{}, fmt.Errorf("marshal audit record: %w", err)
	}
	line = append(line, '\n')
	n, err := w.currentFile.Write(line)
	if err != nil {
		return IndexedRecord{}, fmt.Errorf("write audit record: %w", err)
	}
	if n != len(line) {
		return IndexedRecord{}, fmt.Errorf("short audit write: %d of %d", n, len(line))
	}
	return IndexedRecord{Record: record, FilePath: w.filePath(day), FileOffset: offset, FileLength: int64(len(line)), Line: line}, nil
}

func (w *Writer) openDay(day string) error {
	if w.currentFile != nil && w.currentDay == day {
		return nil
	}
	if w.currentFile != nil {
		if err := w.currentFile.Sync(); err != nil {
			return err
		}
		if err := w.currentFile.Close(); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(w.filePath(day), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open audit file: %w", err)
	}
	w.currentDay = day
	w.currentFile = file
	return nil
}

func (w *Writer) filePath(day string) string {
	return filepath.Join(w.dir, day+".jsonl")
}

func (w *Writer) setErr(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err == nil {
		w.err = err
	}
}
