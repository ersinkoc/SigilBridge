package cliacp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

type ProcessConfig struct {
	Command     string
	Args        []string
	Env         []string
	Framing     string
	IdleTimeout time.Duration
}

type Process struct {
	id      string
	cmd     *exec.Cmd
	codec   *Codec
	stderr  *StderrRing
	done    chan struct{}
	waitErr error
	closed  atomic.Bool
	nextID  atomic.Int64
	callMu  sync.Mutex
	idle    *time.Timer
	timeout time.Duration
}

func StartProcess(ctx context.Context, id string, cfg ProcessConfig) (*Process, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("cli command is required")
	}
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Env = append(cmd.Environ(), cfg.Env...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	ring := NewStderrRing(4096)
	go func() {
		_, _ = io.Copy(ring, stderrPipe)
	}()
	rwc := readWriteCloser{Reader: stdout, Writer: stdin, Closer: stdin}
	codec := NewCodec(rwc)
	if cfg.Framing == "ndjson" {
		codec = NewNDJSONCodec(rwc)
	}
	proc := &Process{id: id, cmd: cmd, codec: codec, stderr: ring, done: make(chan struct{}), timeout: cfg.IdleTimeout}
	go func() {
		proc.waitErr = cmd.Wait()
		proc.closed.Store(true)
		close(proc.done)
	}()
	proc.touch()
	return proc, nil
}

func (p *Process) Call(ctx context.Context, method string, params any, result any) error {
	return p.CallWithHandler(ctx, method, params, result, nil)
}

func (p *Process) CallWithHandler(ctx context.Context, method string, params any, result any, handler func(Message) error) error {
	p.callMu.Lock()
	defer p.callMu.Unlock()
	p.touch()
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	id := p.nextID.Add(1)
	if err := p.codec.Send(Message{ID: id, Method: method, Params: raw}); err != nil {
		return err
	}
	for {
		message, err := p.recv(ctx)
		if err != nil {
			return err
		}
		if !sameID(message.ID, float64(id)) && !sameID(message.ID, id) {
			if err := p.handlePeerMessage(message, handler); err != nil {
				return err
			}
			continue
		}
		if message.Error != nil {
			return fmt.Errorf("json-rpc error %d: %s", message.Error.Code, message.Error.Message)
		}
		if result == nil {
			return nil
		}
		return json.Unmarshal(message.Result, result)
	}
}

func (p *Process) handlePeerMessage(message Message, handler func(Message) error) error {
	if handler != nil {
		if err := handler(message); err != nil {
			return err
		}
	}
	if message.Method == "" || message.ID == nil {
		return nil
	}
	return p.codec.Send(Message{
		ID: message.ID,
		Error: &Error{
			Code:    -32601,
			Message: fmt.Sprintf("method %q is not supported by SigilBridge ACP client", message.Method),
		},
	})
}

func (p *Process) Shutdown(ctx context.Context) error {
	if p.closed.Load() {
		return nil
	}
	_ = p.codec.Send(Message{ID: p.nextID.Add(1), Method: MethodShutdown})
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-p.done:
		p.closed.Store(true)
		return nil
	case <-timer.C:
		_ = p.cmd.Process.Kill()
		p.closed.Store(true)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Process) Alive() bool {
	return !p.closed.Load()
}

func (p *Process) Stderr() string {
	return p.stderr.String()
}

func (p *Process) recv(ctx context.Context) (Message, error) {
	type result struct {
		msg Message
		err error
	}
	ch := make(chan result, 1)
	go func() {
		msg, err := p.codec.Recv()
		ch <- result{msg: msg, err: err}
	}()
	select {
	case out := <-ch:
		return out.msg, out.err
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

func (p *Process) touch() {
	if p.timeout <= 0 {
		return
	}
	if p.idle == nil {
		p.idle = time.AfterFunc(p.timeout, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = p.Shutdown(ctx)
		})
		return
	}
	p.idle.Reset(p.timeout)
}

type readWriteCloser struct {
	io.Reader
	io.Writer
	io.Closer
}

func sameID(left any, right any) bool {
	return fmt.Sprint(left) == fmt.Sprint(right)
}
