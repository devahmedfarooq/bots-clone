package registry

import (
	"bots/internal/config"
	"bots/internal/lock"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func (r Registry) Replace(b config.Bot) (config.Bot, error) {
	if e := b.Validate(); e != nil {
		return b, e
	}
	if e := os.MkdirAll(r.Dir, 0700); e != nil {
		return b, e
	}
	unlock, e := lock.Acquire(filepath.Join(r.Dir, "lifecycle.lock"))
	if e != nil {
		return b, e
	}
	defer unlock()
	current, e := r.Get(b.ID)
	if e != nil {
		return b, e
	}
	if current.Version == b.Version {
		return b, errors.New("upgrade requires a different version")
	}
	for index, v := range []config.Bot{current, b} {
		dir := filepath.Join(r.Dir, "versions", v.ID)
		if e = os.MkdirAll(dir, 0700); e != nil {
			return b, e
		}
		p := filepath.Join(dir, v.Version+".json")
		data, _ := json.MarshalIndent(v, "", "  ")
		if old, e := os.ReadFile(p); e == nil {
			if index == 1 && string(old) != string(data) {
				return b, errors.New("version already exists with different contents")
			}
		} else if !os.IsNotExist(e) {
			return b, e
		} else if e = os.WriteFile(p, data, 0600); e != nil {
			return b, e
		}
	}
	p, _ := r.path(b.ID)
	data, _ := json.MarshalIndent(b, "", "  ")
	f, e := os.CreateTemp(r.Dir, ".replace-")
	if e != nil {
		return b, e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(data); e != nil {
		f.Close()
		return b, e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return b, e
	}
	if e = f.Close(); e != nil {
		return b, e
	}
	return b, os.Rename(f.Name(), p)
}
func (r Registry) Previous(id, version string) (config.Bot, error) {
	if !config.ValidID(id) {
		return config.Bot{}, errors.New("invalid bot ID")
	}
	var test = config.Bot{Schema: 1, ID: id, Version: version, Description: "x", Instructions: "x", BaseModel: "x", Budgets: config.Budgets{Tokens: 1, Seconds: 1}}
	if e := test.Validate(); e != nil {
		return config.Bot{}, e
	}
	return config.Load(filepath.Join(r.Dir, "versions", id, version+".json"))
}
