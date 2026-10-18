package application

import (
	"errors"
	"path/filepath"

	"bots/internal/config"
	"bots/internal/registry"
	"bots/internal/tools"
)

type Catalog struct {
	Registry registry.Registry
	Model    string
}

func (c Catalog) Validate(dir string) (config.Bot, error) {
	b, err := config.Load(filepath.Join(dir, "bot.json"))
	if err != nil {
		return b, err
	}
	if b.BaseModel != c.Model {
		return b, errors.New("base_model does not match configured engine")
	}
	r, close, err := tools.Files(b.ContextSources)
	if err != nil {
		return b, err
	}
	defer close()
	_, err = r.Definitions(b.Tools)
	return b, err
}

func (c Catalog) Install(dir string) (config.Bot, error) {
	b, err := c.Validate(dir)
	if err != nil {
		return b, err
	}
	return c.Registry.Register(dir)
}
