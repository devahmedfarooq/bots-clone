package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"bots/internal/artifacts"
)

type AdapterProvider interface {
	ValidateAdapter(context.Context, string) error
	LoadAdapter(context.Context, string) error
	ActivateAdapter(context.Context, string) error
	DeactivateAdapter(context.Context) error
	UnloadAdapter(context.Context, string) error
}
type loadedAdapter struct {
	path     string
	manifest artifacts.Adapter
}
type adapterScale struct {
	ID    int     `json:"id"`
	Scale float64 `json:"scale"`
}

func (p *Llama) ValidateAdapter(ctx context.Context, path string) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if p.BaseManifest == "" || p.RuntimeRevision == "" {
		return errors.New("configure engine.base_manifest and engine.runtime_revision before loading adapters")
	}
	b, e := artifacts.LoadBase(p.BaseManifest)
	if e != nil {
		return fmt.Errorf("adapter base identity: %w", e)
	}
	a, e := artifacts.LoadAdapter(path)
	if e != nil {
		return e
	}
	if e = a.Compatible(b, p.RuntimeRevision); e != nil {
		return e
	}
	if e = artifacts.Verify(p.Binary, a.RuntimeSHA256); e != nil {
		return fmt.Errorf("runtime artifact: %w", e)
	}
	probe, cancel := context.WithTimeout(ctx, 10*time.Second)
	version, probeErr := exec.CommandContext(probe, p.Binary, "--version").CombinedOutput()
	cancel()
	if probeErr != nil || !strings.Contains(string(version), "build "+strings.TrimPrefix(a.RuntimeRevision, "b")+",") {
		return fmt.Errorf("runtime does not report pinned build %s", a.RuntimeRevision)
	}
	if e = artifacts.Verify(p.ModelPath, b.SHA256); e != nil {
		return fmt.Errorf("base artifact: %w", e)
	}
	return a.Verify()
}
func (p *Llama) prepareAdapters() error {
	loaded := []loadedAdapter{}
	seen := map[string]bool{}
	for _, path := range p.AdapterPaths {
		path, e := filepath.Abs(path)
		if e != nil {
			return e
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		if e = p.ValidateAdapter(context.Background(), path); e != nil {
			return e
		}
		a, e := artifacts.LoadAdapter(path)
		if e != nil {
			return e
		}
		loaded = append(loaded, loadedAdapter{path, a})
	}
	p.adapters = loaded
	return nil
}
func (p *Llama) index(path string) int {
	for i, a := range p.adapters {
		if a.path == path {
			return i
		}
	}
	return -1
}
func (p *Llama) LoadAdapter(ctx context.Context, path string) error {
	path, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	if e = p.ValidateAdapter(ctx, path); e != nil {
		return e
	}
	if index := p.index(path); index >= 0 {
		current, e := artifacts.LoadAdapter(path)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(current, p.adapters[index].manifest) {
			return errors.New("loaded adapter manifest changed; restart provider")
		}
		return nil
	}
	model := Model{ID: p.ModelID, ContextSize: p.loadedContext}
	if model.ContextSize == 0 {
		model.ContextSize = p.Capacity
	}
	if e = p.Unload(ctx); e != nil {
		return e
	}
	previous := append([]string{}, p.AdapterPaths...)
	p.AdapterPaths = append(p.AdapterPaths, path)
	if e = p.Load(ctx, model); e != nil {
		p.AdapterPaths = previous
		return fmt.Errorf("adapter pool load failed (base will reload on next run): %w", e)
	}
	return nil
}
func (p *Llama) scales(path string) []adapterScale {
	out := make([]adapterScale, len(p.adapters))
	for i, a := range p.adapters {
		out[i].ID = i
		if a.path == path {
			out[i].Scale = 1
		}
	}
	return out
}
func (p *Llama) erase(ctx context.Context) error {
	res, e := p.post(ctx, "/slots/0?action=erase", map[string]any{})
	if e != nil {
		return e
	}
	defer res.Body.Close()
	_, e = io.Copy(io.Discard, res.Body)
	return e
}
func (p *Llama) setAdapters(ctx context.Context, path string) error {
	res, e := p.post(ctx, "/lora-adapters", p.scales(path))
	if e != nil {
		return e
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	req, e := http.NewRequestWithContext(ctx, "GET", p.URL+"/lora-adapters", nil)
	if e != nil {
		return e
	}
	res, e = p.client().Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("cannot verify active adapter")
	}
	var got []struct {
		adapterScale
		Path string `json:"path"`
	}
	if e = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&got); e != nil {
		return e
	}
	expected := p.scales(path)
	if len(got) != len(expected) {
		return errors.New("adapter pool mismatch")
	}
	seen := map[int]bool{}
	for _, a := range got {
		if a.ID < 0 || a.ID >= len(expected) || seen[a.ID] || a.Scale != expected[a.ID].Scale {
			return errors.New("engine adapter state mismatch")
		}
		if !strings.EqualFold(filepath.Clean(a.Path), filepath.Clean(p.adapters[a.ID].manifest.Artifact)) {
			return errors.New("engine adapter artifact path mismatch")
		}
		seen[a.ID] = true
	}
	return nil
}
func (p *Llama) recoverAdapter(cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e := p.setAdapters(ctx, "")
	if e == nil {
		e = p.erase(ctx)
	}
	p.active = ""
	if e != nil {
		return errors.Join(cause, fmt.Errorf("adapter recovery: %w", e), p.Unload(ctx))
	}
	return cause
}
func (p *Llama) selectAdapter(ctx context.Context, path string) error {
	if len(p.adapters) == 0 && path == "" {
		return nil
	}
	if e := p.erase(ctx); e != nil {
		return p.recoverAdapter(e)
	}
	if e := p.setAdapters(ctx, path); e != nil {
		return p.recoverAdapter(e)
	}
	p.active = path
	return nil
}
func (p *Llama) ActivateAdapter(ctx context.Context, path string) error {
	path, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	if p.index(path) < 0 {
		return errors.New("adapter is not loaded")
	}
	if e = p.LoadAdapter(ctx, path); e != nil {
		return e
	}
	return p.selectAdapter(ctx, path)
}
func (p *Llama) DeactivateAdapter(ctx context.Context) error { return p.selectAdapter(ctx, "") }
func (p *Llama) UnloadAdapter(ctx context.Context, path string) error {
	path, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	if e = p.Unload(ctx); e != nil {
		return e
	}
	next := []string{}
	for _, v := range p.AdapterPaths {
		a, _ := filepath.Abs(v)
		if a != path {
			next = append(next, v)
		}
	}
	p.AdapterPaths = next
	p.adapters = nil
	return nil
}
