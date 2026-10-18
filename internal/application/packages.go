package application

import (
	"bots/internal/config"
	"bots/internal/packages"
)

func (c Catalog) Import(path, home string) (config.Bot, error) {
	dir, cleanup, err := packages.Extract(path, home)
	if err != nil {
		return config.Bot{}, err
	}
	b, err := c.Install(dir)
	if err != nil {
		cleanup()
	}
	return b, err
}
