package main

import (
	"bots/internal/application"
	"bots/internal/artifacts"
	"bots/internal/config"
	"bots/internal/packages"
	"errors"
	"path/filepath"
)

func packageCommand(c appConfig, catalog application.Catalog, args []string) error {
	if len(args) < 2 {
		return errors.New("package install/validate/upgrade ZIP | rollback ID VERSION | export ID ZIP | uninstall ID")
	}
	op := args[0]
	if op == "install" || op == "validate" || op == "upgrade" {
		if len(args) != 2 {
			return errors.New("expected ZIP path")
		}
		dir, clean, e := packages.Extract(args[1], filepath.Join(c.Home, "packages"))
		if e != nil {
			return e
		}
		keep := false
		defer func() {
			if !keep {
				clean()
			}
		}()
		if op == "validate" {
			b, e := catalog.Validate(dir)
			if e != nil {
				return e
			}
			return write(b)
		}
		if op == "upgrade" {
			b, e := catalog.Upgrade(dir)
			if e != nil {
				return e
			}
			keep = true
			return write(b)
		}
		b, e := catalog.Install(dir)
		if e != nil {
			return e
		}
		keep = true
		return write(b)
	}
	if op == "uninstall" && len(args) == 2 {
		if e := catalog.Registry.Remove(args[1]); e != nil {
			return e
		}
		return write(map[string]string{"removed": args[1]})
	}
	if len(args) != 3 {
		return errors.New("expected ID and version/export path")
	}
	var b config.Bot
	var e error
	switch op {
	case "rollback":
		b, e = catalog.Rollback(args[1], args[2])
	case "export":
		b, e = catalog.Registry.Get(args[1])
		if e == nil {
			e = packages.Export(b, args[2])
		}
	default:
		e = errors.New("unknown package operation")
	}
	if e != nil {
		return e
	}
	return write(b)
}
func artifactCommand(c appConfig, args []string) error {
	if len(args) != 3 || args[0] != "import" {
		return errors.New("artifact import FILE SHA256")
	}
	p, e := (artifacts.Store{Dir: filepath.Join(c.Home, "artifacts")}).Import(args[1], args[2])
	if e != nil {
		return e
	}
	return write(map[string]string{"artifact": p})
}
