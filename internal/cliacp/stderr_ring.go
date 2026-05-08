package cliacp

import "sync"

type StderrRing struct {
	mu   sync.Mutex
	buf  []byte
	next int
	full bool
}

func NewStderrRing(size int) *StderrRing {
	if size <= 0 {
		size = 4096
	}
	return &StderrRing{buf: make([]byte, size)}
}

func (r *StderrRing) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range p {
		r.buf[r.next] = b
		r.next = (r.next + 1) % len(r.buf)
		if r.next == 0 {
			r.full = true
		}
	}
	return len(p), nil
}

func (r *StderrRing) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		return string(r.buf[:r.next])
	}
	out := append([]byte(nil), r.buf[r.next:]...)
	out = append(out, r.buf[:r.next]...)
	return string(out)
}
