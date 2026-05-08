package strategy

import (
	"math/rand"
	"testing"
)

func TestStrategies(t *testing.T) {
	candidates := []Candidate{
		{ID: "a", Weight: 1, Priority: 2, Healthy: true, InFlight: 5, LatencyMs: 20},
		{ID: "b", Weight: 10, Priority: 1, Healthy: true, InFlight: 1, LatencyMs: 30},
		{ID: "c", Weight: 1, Priority: 3, Healthy: true, InFlight: 2, LatencyMs: 10},
	}
	if got, _ := (Priority{}).Select(candidates); got.ID != "b" {
		t.Fatalf("priority = %s", got.ID)
	}
	if got, _ := (LeastInFlight{}).Select(candidates); got.ID != "b" {
		t.Fatalf("least = %s", got.ID)
	}
	if got, _ := (LowestLatency{}).Select(candidates); got.ID != "c" {
		t.Fatalf("latency = %s", got.ID)
	}
	rr := &RoundRobin{}
	first, _ := rr.Select(candidates)
	second, _ := rr.Select(candidates)
	if first.ID != "a" || second.ID != "b" {
		t.Fatalf("round robin = %s, %s", first.ID, second.ID)
	}
	weighted := Weighted{Rand: rand.New(rand.NewSource(1))}
	if got, err := weighted.Select(candidates); err != nil || got.ID == "" {
		t.Fatalf("weighted got=%#v err=%v", got, err)
	}
	random := Random{Rand: rand.New(rand.NewSource(2))}
	if got, err := random.Select(candidates); err != nil || got.ID == "" {
		t.Fatalf("random got=%#v err=%v", got, err)
	}
	if _, err := (FirstAvailable{}).Select([]Candidate{{ID: "x", Healthy: false}}); err == nil {
		t.Fatalf("expected no candidate error")
	}
}

func TestNewStrategies(t *testing.T) {
	for _, name := range []string{"weighted", "weighted_random", "round_robin", "weighted_round_robin", "random", "least_inflight", "least_used", "lowest_latency", "first_available", "priority", "priority_first"} {
		if New(name) == nil {
			t.Fatalf("New(%q) returned nil", name)
		}
	}
}
