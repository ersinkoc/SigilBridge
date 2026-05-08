package strategy

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
)

type Candidate struct {
	ID        string
	Weight    int
	Priority  int
	Healthy   bool
	InFlight  int
	LatencyMs int64
}

type Selector interface {
	Select([]Candidate) (Candidate, error)
}

func healthy(candidates []Candidate) []Candidate {
	out := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Healthy {
			if c.Weight <= 0 {
				c.Weight = 1
			}
			out = append(out, c)
		}
	}
	return out
}

func noCandidate(candidates []Candidate) (Candidate, error) {
	if len(candidates) == 0 {
		return Candidate{}, fmt.Errorf("no healthy upstream candidates")
	}
	return Candidate{}, nil
}

type Priority struct{}

func (Priority) Select(candidates []Candidate) (Candidate, error) {
	candidates = healthy(candidates)
	if _, err := noCandidate(candidates); err != nil {
		return Candidate{}, err
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.Priority < best.Priority {
			best = c
		}
	}
	return best, nil
}

type Weighted struct {
	Intn func(int) (int, error)
}

func (s Weighted) Select(candidates []Candidate) (Candidate, error) {
	candidates = healthy(candidates)
	if _, err := noCandidate(candidates); err != nil {
		return Candidate{}, err
	}
	total := 0
	for _, c := range candidates {
		total += c.Weight
	}
	rnd, err := cryptoIntn(total)
	if s.Intn != nil {
		rnd, err = s.Intn(total)
	}
	if err != nil {
		return Candidate{}, err
	}
	for _, c := range candidates {
		if rnd < c.Weight {
			return c, nil
		}
		rnd -= c.Weight
	}
	return candidates[len(candidates)-1], nil
}

type RoundRobin struct {
	mu   sync.Mutex
	next int
}

func (s *RoundRobin) Select(candidates []Candidate) (Candidate, error) {
	candidates = healthy(candidates)
	if _, err := noCandidate(candidates); err != nil {
		return Candidate{}, err
	}
	s.mu.Lock()
	idx := s.next % len(candidates)
	s.next = (s.next + 1) % len(candidates)
	s.mu.Unlock()
	return candidates[idx], nil
}

type Random struct {
	Intn func(int) (int, error)
}

func (s Random) Select(candidates []Candidate) (Candidate, error) {
	candidates = healthy(candidates)
	if _, err := noCandidate(candidates); err != nil {
		return Candidate{}, err
	}
	idx, err := cryptoIntn(len(candidates))
	if s.Intn != nil {
		idx, err = s.Intn(len(candidates))
	}
	if err != nil {
		return Candidate{}, err
	}
	return candidates[idx], nil
}

type LeastInFlight struct{}

func (LeastInFlight) Select(candidates []Candidate) (Candidate, error) {
	candidates = healthy(candidates)
	if _, err := noCandidate(candidates); err != nil {
		return Candidate{}, err
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.InFlight < best.InFlight {
			best = c
		}
	}
	return best, nil
}

type LowestLatency struct{}

func (LowestLatency) Select(candidates []Candidate) (Candidate, error) {
	candidates = healthy(candidates)
	if _, err := noCandidate(candidates); err != nil {
		return Candidate{}, err
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.LatencyMs < best.LatencyMs {
			best = c
		}
	}
	return best, nil
}

type FirstAvailable struct{}

func (FirstAvailable) Select(candidates []Candidate) (Candidate, error) {
	candidates = healthy(candidates)
	if _, err := noCandidate(candidates); err != nil {
		return Candidate{}, err
	}
	return candidates[0], nil
}

func New(name string) Selector {
	switch name {
	case "weighted", "weighted_random":
		return Weighted{}
	case "round_robin", "weighted_round_robin":
		return &RoundRobin{}
	case "random":
		return Random{}
	case "least_inflight", "least_used":
		return LeastInFlight{}
	case "lowest_latency":
		return LowestLatency{}
	case "first_available":
		return FirstAvailable{}
	case "priority", "priority_first":
		return Priority{}
	default:
		return Priority{}
	}
}

func cryptoIntn(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("random upper bound must be positive")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, fmt.Errorf("secure random selection: %w", err)
	}
	return int(n.Int64()), nil
}
