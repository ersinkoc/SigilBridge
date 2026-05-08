package adapter

import "fmt"

type Registry struct {
	providers map[string]Provider
}

func NewRegistry(providers ...Provider) (*Registry, error) {
	r := &Registry{providers: map[string]Provider{}}
	for _, provider := range providers {
		if err := r.Register(provider); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Registry) Register(provider Provider) error {
	if provider == nil {
		return fmt.Errorf("provider is nil")
	}
	id := provider.ID()
	if id == "" {
		return fmt.Errorf("provider id is required")
	}
	if _, exists := r.providers[id]; exists {
		return fmt.Errorf("provider %q already registered", id)
	}
	r.providers[id] = provider
	return nil
}

func (r *Registry) Get(id string) (Provider, error) {
	provider, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("provider %q is not registered", id)
	}
	return provider, nil
}

func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}
