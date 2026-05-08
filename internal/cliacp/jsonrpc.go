package cliacp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Codec struct {
	r       *bufio.Reader
	w       io.Writer
	wc      io.Closer
	mu      sync.Mutex
	framing string
}

func NewCodec(rwc io.ReadWriteCloser) *Codec {
	return &Codec{r: bufio.NewReader(rwc), w: rwc, wc: rwc}
}

func NewNDJSONCodec(rwc io.ReadWriteCloser) *Codec {
	return &Codec{r: bufio.NewReader(rwc), w: rwc, wc: rwc, framing: "ndjson"}
}

func (c *Codec) Send(message Message) error {
	if message.JSONRPC == "" {
		message.JSONRPC = "2.0"
	}
	raw, err := json.Marshal(message)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.framing == "ndjson" {
		if _, err := c.w.Write(raw); err != nil {
			return err
		}
		_, err = c.w.Write([]byte("\n"))
		return err
	}
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(raw)); err != nil {
		return err
	}
	_, err = c.w.Write(raw)
	return err
}

func (c *Codec) Recv() (Message, error) {
	if c.framing == "ndjson" {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return Message{}, err
		}
		return parseMessage([]byte(strings.TrimSpace(line)))
	}
	length := -1
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return Message{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return Message{}, fmt.Errorf("malformed JSON-RPC frame header %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || parsed < 0 {
				return Message{}, fmt.Errorf("invalid Content-Length %q", strings.TrimSpace(value))
			}
			length = parsed
		}
	}
	if length < 0 {
		return Message{}, fmt.Errorf("missing Content-Length header")
	}
	raw := make([]byte, length)
	if _, err := io.ReadFull(c.r, raw); err != nil {
		return Message{}, err
	}
	return parseMessage(raw)
}

func parseMessage(raw []byte) (Message, error) {
	var message Message
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&message); err != nil {
		return Message{}, fmt.Errorf("parse JSON-RPC message: %w", err)
	}
	if message.JSONRPC != "2.0" {
		return Message{}, fmt.Errorf("unsupported JSON-RPC version %q", message.JSONRPC)
	}
	return message, nil
}

func (c *Codec) Close() error {
	if c.wc == nil {
		return nil
	}
	return c.wc.Close()
}
