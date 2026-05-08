package strategy

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"
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
	Rand *rand.Rand
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
	rnd := rand.Intn(total)
	if s.Rand != nil {
		rnd = s.Rand.Intn(total)
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
	next atomic.Uint64
}

func (s *RoundRobin) Select(candidates []Candidate) (Candidate, error) {
	candidates = healthy(candidates)
	if _, err := noCandidate(candidates); err != nil {
		return Candidate{}, err
	}
	idx := int(s.next.Add(1)-1) % len(candidates)
	return candidates[idx], nil
}

type Random struct {
	Rand *rand.Rand
}

func (s Random) Select(candidates []Candidate) (Candidate, error) {
	candidates = healthy(candidates)
	if _, err := noCandidate(candidates); err != nil {
		return Candidate{}, err
	}
	if s.Rand != nil {
		return candidates[s.Rand.Intn(len(candidates))], nil
	}
	return candidates[rand.Intn(len(candidates))], nil
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
		return Weighted{Rand: rand.New(rand.NewSource(time.Now().UnixNano()))}
	case "round_robin", "weighted_round_robin":
		return &RoundRobin{}
	case "random":
		return Random{Rand: rand.New(rand.NewSource(time.Now().UnixNano()))}
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
