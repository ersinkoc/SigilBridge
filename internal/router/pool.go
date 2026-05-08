package router

import (
	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/router/strategy"
)

type Upstream struct {
	ID       string
	Priority int
	Weight   int
	Provider adapter.Provider
	Config   adapter.ProviderConfig
}

type Pool struct {
	ID           string
	ModelAliases []string
	Strategy     string
	Upstreams    []Upstream
}

type Selection struct {
	Pool     Pool
	Upstream Upstream
}

func (p Pool) candidates(health map[string]*Health) []strategy.Candidate {
	out := make([]strategy.Candidate, 0, len(p.Upstreams))
	for _, upstream := range p.Upstreams {
		h := health[upstream.ID]
		available := h == nil || h.Available()
		out = append(out, strategy.Candidate{ID: upstream.ID, Priority: upstream.Priority, Weight: upstream.Weight, Healthy: available})
	}
	return out
}
