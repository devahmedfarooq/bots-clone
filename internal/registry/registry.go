package registry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"bots/internal/config"
)

type Registry struct{ Dir string }

func (r Registry) path(id string) (string, error) {
	if !config.ValidID(id) {
		return "", errors.New("invalid bot ID")
	}
	return filepath.Join(r.Dir, id+".json"), nil
}
func (r Registry) Register(dir string) (config.Bot, error) {
	b, err := config.Load(filepath.Join(dir, "bot.json"))
	if err != nil {
		return b, err
	}
	return r.RegisterBot(b)
}
func (r Registry) RegisterBot(b config.Bot) (config.Bot, error) {
	if err := b.Validate(); err != nil {
		return b, err
	}
	var err error
	p, _ := r.path(b.ID)
	if err = os.MkdirAll(r.Dir, 0700); err != nil {
		return b, err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return b, err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return b, err
	}
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		os.Remove(p)
		return b, err
	}
	return b, closeErr
}
func (r Registry) Get(id string) (config.Bot, error) {
	p, e := r.path(id)
	if e != nil {
		return config.Bot{}, e
	}
	return config.Load(p)
}
func (r Registry) Remove(id string) error {
	p, e := r.path(id)
	if e != nil {
		return e
	}
	return os.Remove(p)
}
func (r Registry) List() ([]config.Bot, error) {
	out := []config.Bot{}
	entries, e := os.ReadDir(r.Dir)
	if os.IsNotExist(e) {
		return out, nil
	}
	if e != nil {
		return nil, e
	}
	for _, f := range entries {
		if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
			continue
		}
		b, e := r.Get(f.Name()[:len(f.Name())-5])
		if e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
