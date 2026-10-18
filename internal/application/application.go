package application

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"bots/internal/artifacts"
	"bots/internal/config"
	"bots/internal/registry"
	"bots/internal/tools"
)

type Catalog struct {
	Registry        registry.Registry
	Model           string
	BaseManifest    string
	RuntimeRevision string
	ArtifactDir     string
	RuntimeBinary   string
	Plugins         []tools.Tool
}

func (c Catalog) Validate(dir string) (config.Bot, error) {
	b, err := config.Load(filepath.Join(dir, "bot.json"))
	if err != nil {
		return b, err
	}
	return b, c.Check(b)
}

func (c Catalog) Check(b config.Bot) error {
	if err := b.Validate(); err != nil {
		return err
	}
	if b.BaseModel != c.Model {
		return errors.New("base_model does not match configured engine")
	}
	if b.Adapter != "" {
		a, e := c.adapter(b.Adapter)
		if e != nil {
			return e
		}
		base, e := artifacts.LoadBase(c.BaseManifest)
		if e != nil {
			return e
		}
		if e = a.Compatible(base, c.RuntimeRevision); e != nil {
			return e
		}
		if e = artifacts.Verify(c.RuntimeBinary, a.RuntimeSHA256); e != nil {
			return e
		}
		if e = a.Verify(); e != nil {
			return e
		}
	}
	r, close, err := tools.WithPlugins(b.ContextSources, c.Plugins)
	if err != nil {
		return err
	}
	defer close()
	_, err = r.Definitions(b.Tools)
	return err
}

func (c Catalog) prepare(dir string) (config.Bot, error) {
	b, err := c.Validate(dir)
	if err != nil {
		return b, err
	}
	if b.Adapter != "" {
		a, e := c.adapter(b.Adapter)
		if e != nil {
			return b, e
		}
		data, e := json.MarshalIndent(a, "", "  ")
		if e != nil {
			return b, e
		}
		home := filepath.Join(c.ArtifactDir, "manifests")
		if e = os.MkdirAll(home, 0700); e != nil {
			return b, e
		}
		path := filepath.Join(home, a.ID+"-"+a.Version+"-"+a.SHA256+".json")
		if old, e := os.ReadFile(path); e == nil {
			if string(old) != string(data) {
				return b, errors.New("adapter manifest identity already exists with different metadata")
			}
		} else if !os.IsNotExist(e) {
			return b, e
		} else if e = os.WriteFile(path, data, 0600); e != nil {
			return b, e
		}
		b.Adapter = path
	}
	return b, nil
}

func (c Catalog) adapter(path string) (artifacts.Adapter, error) {
	a, e := artifacts.LoadAdapter(path)
	if e != nil {
		return a, e
	}
	if strings.HasPrefix(a.Artifact, "sha256:") {
		if a.Artifact != "sha256:"+a.SHA256 {
			return a, errors.New("artifact reference and checksum mismatch")
		}
		a.Artifact, e = (artifacts.Store{Dir: c.ArtifactDir}).Resolve(a.SHA256)
	}
	return a, e
}

func (c Catalog) Install(dir string) (config.Bot, error) {
	b, e := c.prepare(dir)
	if e != nil {
		return b, e
	}
	return c.Registry.RegisterBot(b)
}
func (c Catalog) Upgrade(dir string) (config.Bot, error) {
	b, e := c.prepare(dir)
	if e != nil {
		return b, e
	}
	return c.Registry.Replace(b)
}
func (c Catalog) Rollback(id, version string) (config.Bot, error) {
	b, e := c.Registry.Previous(id, version)
	if e == nil {
		e = c.Check(b)
	}
	if e != nil {
		return b, e
	}
	return c.Registry.Replace(b)
}
