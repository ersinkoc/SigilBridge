package config

import "sync/atomic"

type Reloader struct {
	path     string
	snapshot atomic.Value
}

func NewReloader(path string) (*Reloader, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	r := &Reloader{path: path}
	r.snapshot.Store(cfg)
	return r, nil
}

func (r *Reloader) Current() *Config {
	cfg, _ := r.snapshot.Load().(*Config)
	return cfg
}

func (r *Reloader) Reload() (*Config, error) {
	cfg, err := Load(r.path)
	if err != nil {
		return nil, err
	}
	r.snapshot.Store(cfg)
	return cfg, nil
}
