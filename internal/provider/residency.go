package provider

import (
	"context"
	"errors"
)

type Manager struct {
	Provider Provider
	slot     chan struct{}
	closed   bool
}

func NewManager(p Provider) *Manager {
	m := &Manager{Provider: p, slot: make(chan struct{}, 1)}
	m.slot <- struct{}{}
	return m
}
func (m *Manager) Acquire(ctx context.Context, model Model) (func(), error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.slot:
	}
	release := func() { m.slot <- struct{}{} }
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	if m.closed {
		release()
		return nil, errors.New("model manager is closed")
	}
	if err := m.Provider.Load(ctx, model); err != nil {
		release()
		return nil, err
	}
	if model.Adapter != "" {
		p, ok := m.Provider.(AdapterProvider)
		if !ok {
			release()
			return nil, errors.New("provider does not support adapters")
		}
		if err := p.LoadAdapter(ctx, model.Adapter); err != nil {
			release()
			return nil, err
		}
		if err := p.ActivateAdapter(ctx, model.Adapter); err != nil {
			release()
			return nil, err
		}
	} else if p, ok := m.Provider.(AdapterProvider); ok {
		if err := p.DeactivateAdapter(ctx); err != nil {
			release()
			return nil, err
		}
	}
	return release, nil
}
func (m *Manager) Close(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.slot:
	}
	defer func() { m.slot <- struct{}{} }()
	m.closed = true
	return m.Provider.Unload(ctx)
}
