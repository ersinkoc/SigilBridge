package router

import (
	"context"
	"fmt"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/ir"
	"github.com/sigilbridge/sigilbridge/internal/router/strategy"
)

type Router struct {
	pools    map[string]Pool
	health   map[string]*Health
	breakers map[string]*Breaker
}

func New(pools []Pool) *Router {
	r := &Router{pools: map[string]Pool{}, health: map[string]*Health{}, breakers: map[string]*Breaker{}}
	for _, pool := range pools {
		for _, alias := range pool.ModelAliases {
			r.pools[alias] = pool
		}
		for _, upstream := range pool.Upstreams {
			if _, ok := r.health[upstream.ID]; !ok {
				h := NewHealth()
				r.health[upstream.ID] = &h
			}
			r.breakers[upstream.ID] = NewBreaker(2, 50*time.Millisecond)
		}
	}
	return r
}

func (r *Router) Resolve(modelAlias string) (Pool, error) {
	pool, ok := r.pools[modelAlias]
	if !ok {
		return Pool{}, fmt.Errorf("no pool for model alias %q", modelAlias)
	}
	return pool, nil
}

func (r *Router) Select(pool Pool) (Selection, error) {
	selected, err := strategy.New(pool.Strategy).Select(pool.candidates(r.health))
	if err != nil {
		selected, err = strategy.New(pool.Strategy).Select(r.recoveryCandidates(pool))
	}
	if err != nil {
		return Selection{}, err
	}
	for _, upstream := range pool.Upstreams {
		if upstream.ID == selected.ID {
			return Selection{Pool: pool, Upstream: upstream}, nil
		}
	}
	return Selection{}, fmt.Errorf("selected upstream %q not in pool", selected.ID)
}

func (r *Router) recoveryCandidates(pool Pool) []strategy.Candidate {
	out := make([]strategy.Candidate, 0, len(pool.Upstreams))
	for _, upstream := range pool.Upstreams {
		h := r.health[upstream.ID]
		breaker := r.breakers[upstream.ID]
		available := h != nil && h.State == HealthCoolingOff && (breaker == nil || breaker.Allow())
		out = append(out, strategy.Candidate{ID: upstream.ID, Priority: upstream.Priority, Weight: upstream.Weight, Healthy: available})
	}
	return out
}

func (r *Router) Dispatch(ctx context.Context, req ir.Request) (ir.Response, error) {
	pool, err := r.Resolve(req.ModelAlias)
	if err != nil {
		return ir.Response{}, err
	}
	var last error
	for attempts := 0; attempts < len(pool.Upstreams); attempts++ {
		selection, err := r.Select(pool)
		if err != nil {
			return ir.Response{}, err
		}
		upstream := selection.Upstream
		breaker := r.breakers[upstream.ID]
		if breaker != nil && !breaker.Allow() {
			r.health[upstream.ID].Cooldown()
			last = fmt.Errorf("upstream %q breaker open", upstream.ID)
			continue
		}
		resp, err := upstream.Provider.Chat(ctx, req, upstream.Config)
		if err == nil {
			resp.UpstreamID = upstream.ID
			r.health[upstream.ID].Success()
			if breaker != nil {
				breaker.Success()
			}
			return resp, nil
		}
		last = err
		r.health[upstream.ID].Failure()
		r.health[upstream.ID].Cooldown()
		if breaker != nil {
			breaker.Failure()
		}
		if adapterErr, ok := err.(*adapter.Error); ok && !adapterErr.Retryable {
			return ir.Response{}, err
		}
	}
	if last == nil {
		last = fmt.Errorf("no upstreams attempted")
	}
	return ir.Response{}, last
}

func (r *Router) Stream(ctx context.Context, req ir.Request) (<-chan ir.Event, error) {
	resp, err := r.Dispatch(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan ir.Event)
	go func() {
		defer close(ch)
		ch <- ir.Event{Version: ir.Version, Type: ir.EventStart}
		for i, block := range resp.Content {
			ch <- ir.Event{Version: ir.Version, Type: ir.EventContentBlockDelta, Index: i, Delta: &block}
		}
		ch <- ir.Event{Version: ir.Version, Type: ir.EventUsage, Usage: &resp.Usage}
		ch <- ir.Event{Version: ir.Version, Type: ir.EventStop, StopReason: resp.StopReason}
	}()
	return ch, nil
}

func (r *Router) Health(id string) Health {
	if h := r.health[id]; h != nil {
		return *h
	}
	return Health{State: HealthSick}
}
