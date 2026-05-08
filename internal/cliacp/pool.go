package cliacp

import (
	"context"
	"sync"
)

type Pool struct {
	mu        sync.Mutex
	processes map[string]*Process
	onCrash   func(id string, stderr string)
}

func NewPool(onCrash func(id string, stderr string)) *Pool {
	return &Pool{processes: map[string]*Process{}, onCrash: onCrash}
}

func (p *Pool) Get(ctx context.Context, id string, cfg ProcessConfig) (*Process, error) {
	p.mu.Lock()
	if proc, ok := p.processes[id]; ok && proc.Alive() {
		p.mu.Unlock()
		proc.touch()
		return proc, nil
	}
	p.mu.Unlock()

	proc, err := StartProcess(ctx, id, cfg)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.processes[id] = proc
	p.mu.Unlock()
	go func() {
		<-proc.done
		proc.closed.Store(true)
		p.mu.Lock()
		if p.processes[id] == proc {
			delete(p.processes, id)
		}
		p.mu.Unlock()
		if proc.waitErr != nil && p.onCrash != nil {
			p.onCrash(id, proc.Stderr())
		}
	}()
	return proc, nil
}

func (p *Pool) Drop(ctx context.Context, id string, proc *Process) {
	p.mu.Lock()
	current := p.processes[id]
	if proc == nil || current == proc {
		delete(p.processes, id)
	}
	p.mu.Unlock()
	if proc == nil {
		proc = current
	}
	if proc != nil {
		_ = proc.Shutdown(ctx)
	}
}

func (p *Pool) Close(ctx context.Context) {
	p.mu.Lock()
	procs := make([]*Process, 0, len(p.processes))
	for _, proc := range p.processes {
		procs = append(procs, proc)
	}
	p.processes = map[string]*Process{}
	p.mu.Unlock()
	for _, proc := range procs {
		_ = proc.Shutdown(ctx)
	}
}
