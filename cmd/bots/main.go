package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bots/internal/application"
	"bots/internal/config"
	"bots/internal/diagnostics"
	"bots/internal/evaluation"
	"bots/internal/lock"
	"bots/internal/provider"
	"bots/internal/registry"
	"bots/internal/runner"
	"bots/internal/state"
	"bots/internal/tools"
)

type appConfig struct {
	Home    string   `json:"home"`
	Plugins []string `json:"plugins,omitempty"`
	Engine  struct {
		CacheType       string `json:"cache_type,omitempty"`
		BaseManifest    string `json:"base_manifest,omitempty"`
		RuntimeRevision string `json:"runtime_revision,omitempty"`
		Binary          string `json:"binary"`
		ModelPath       string `json:"model_path"`
		ModelID         string `json:"model_id"`
		Context         int    `json:"context_size"`
		GPU             int    `json:"gpu_layers"`
		Device          string `json:"device"`
	} `json:"engine"`
}

func loadConfig(path string) (appConfig, error) {
	var c appConfig
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = config.Decode(b, &c); e != nil {
		return c, e
	}
	if c.Engine.CacheType != "" && c.Engine.CacheType != "f16" && c.Engine.CacheType != "q8_0" {
		return c, errors.New("engine.cache_type must be f16 or q8_0")
	}
	if c.Home == "" || c.Engine.Binary == "" || c.Engine.ModelPath == "" || c.Engine.ModelID == "" || c.Engine.Context < 512 || c.Engine.Context > 131072 || c.Engine.GPU < 0 {
		return c, errors.New("invalid framework configuration")
	}
	base, e := filepath.Abs(filepath.Dir(path))
	if e != nil {
		return c, e
	}
	for _, p := range []*string{&c.Home, &c.Engine.Binary, &c.Engine.ModelPath} {
		if !filepath.IsAbs(*p) {
			*p = filepath.Join(base, *p)
		}
	}
	if c.Engine.BaseManifest != "" && !filepath.IsAbs(c.Engine.BaseManifest) {
		c.Engine.BaseManifest = filepath.Join(base, c.Engine.BaseManifest)
	}
	for i, p := range c.Plugins {
		if !filepath.IsAbs(p) {
			c.Plugins[i] = filepath.Join(base, p)
		}
	}
	return c, nil
}
func write(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
func flags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(os.Stderr)
	return f
}
func settings(f *flag.FlagSet) *config.Settings {
	s := new(config.Settings)
	positive := func(target *int) func(string) error {
		return func(raw string) error {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 1 {
				return errors.New("must be a positive integer")
			}
			*target = n
			return nil
		}
	}
	f.Func("max-tokens", "override completion tokens", positive(&s.MaxTokens))
	f.Func("context", "override context size", positive(&s.ContextSize))
	f.Func("temperature", "override temperature", func(raw string) error {
		n, e := strconv.ParseFloat(raw, 64)
		if e == nil {
			s.Temperature = &n
		}
		return e
	})
	return s
}
func execute(ctx context.Context, args []string) error {
	root := flags("bots")
	configPath := root.String("config", "framework.json", "framework configuration path")
	if e := root.Parse(args); e != nil {
		return e
	}
	args = root.Args()
	if len(args) == 0 {
		return errors.New("usage: bots [-config file] diagnostics | bot register/import/list/inspect/validate/remove | settings | run | eval | serve | route | cleanup")
	}
	if args[0] == "diagnostics" {
		if len(args) != 1 {
			return errors.New("diagnostics takes no arguments")
		}
		report := diagnostics.Collect()
		c, e := loadConfig(*configPath)
		if e != nil {
			report.Dependencies["framework_configuration"] = e.Error()
		} else {
			for name, path := range map[string]string{"configured_engine": c.Engine.Binary, "configured_model": c.Engine.ModelPath} {
				if _, e = os.Stat(path); e != nil {
					report.Dependencies[name] = "missing: " + path
				} else {
					report.Dependencies[name] = "available: " + path
				}
			}
		}
		return write(report)
	}
	if args[0] == "dataset" {
		return datasetCommand(args[1:])
	}
	c, e := loadConfig(*configPath)
	if e != nil {
		return e
	}
	reg := registry.Registry{Dir: filepath.Join(c.Home, "bots")}
	if args[0] == "cleanup" {
		return cleanup(c, args[1:])
	}
	plugins, e := tools.LoadPlugins(c.Plugins)
	if e != nil {
		return e
	}
	catalog := application.Catalog{Registry: reg, Model: c.Engine.ModelID, Plugins: plugins, BaseManifest: c.Engine.BaseManifest, RuntimeRevision: c.Engine.RuntimeRevision, ArtifactDir: filepath.Join(c.Home, "artifacts"), RuntimeBinary: c.Engine.Binary}
	if args[0] == "package" {
		return packageCommand(c, catalog, args[1:])
	}
	if args[0] == "artifact" {
		return artifactCommand(c, args[1:])
	}
	if args[0] == "route" {
		f := flags("route")
		input := f.String("input", "", "task with explicit @bot mentions")
		bot := f.String("bot", "", "explicit bot ID")
		if e = f.Parse(args[1:]); e != nil {
			return e
		}
		if f.NArg() != 0 {
			return errors.New("unexpected route arguments")
		}
		result, e := catalog.Route(*input, *bot)
		if e != nil {
			return e
		}
		return write(result)
	}
	if args[0] == "serve" {
		return serve(ctx, c, catalog, args[1:])
	}
	if args[0] == "bot" {
		if len(args) < 2 {
			return errors.New("bot requires register/import/list/inspect/validate/remove")
		}
		if args[1] == "list" {
			if len(args) != 2 {
				return errors.New("bot list takes no arguments")
			}
			bots, e := reg.List()
			if e != nil {
				return e
			}
			return write(bots)
		}
		if len(args) != 3 {
			return errors.New("bot command requires one directory or ID")
		}
		switch args[1] {
		case "import":
			b, e := catalog.Import(args[2], filepath.Join(c.Home, "packages"))
			if e != nil {
				return e
			}
			return write(b)
		case "register", "validate":
			b, e := catalog.Validate(args[2])
			if e != nil {
				return e
			}
			if args[1] == "register" {
				b, e = catalog.Install(args[2])
				if e != nil {
					return e
				}
			}
			return write(b)
		case "inspect":
			b, e := reg.Get(args[2])
			if e != nil {
				return e
			}
			return write(b)
		case "remove":
			if e = reg.Remove(args[2]); e != nil {
				return e
			}
			return write(map[string]string{"removed": args[2]})
		default:
			return errors.New("unknown bot subcommand")
		}
	}
	if args[0] == "settings" {
		if len(args) < 2 {
			return errors.New("settings requires bot ID")
		}
		b, e := reg.Get(args[1])
		if e != nil {
			return e
		}
		f := flags("settings")
		overrides := settings(f)
		if e = f.Parse(args[2:]); e != nil {
			return e
		}
		if f.NArg() != 0 {
			return errors.New("unexpected settings arguments")
		}
		effective, e := config.Resolve(b.Settings, *overrides)
		if e != nil {
			return e
		}
		return write(effective)
	}
	if args[0] != "run" && args[0] != "eval" && args[0] != "compare" {
		return errors.New("unknown command")
	}
	store, e := state.Open(filepath.Join(c.Home, "metadata.db"))
	if e != nil {
		return e
	}
	defer store.Close()
	if args[0] == "run" && len(args) == 3 && args[1] == "show" {
		r, e := store.Get(args[2])
		if e != nil {
			return e
		}
		return write(r)
	}
	unlock, e := lock.Acquire(filepath.Join(c.Home, "inference.lock"))
	if e != nil {
		return e
	}
	defer unlock()
	p := &provider.Llama{Binary: c.Engine.Binary, ModelPath: c.Engine.ModelPath, ModelID: c.Engine.ModelID, BaseManifest: c.Engine.BaseManifest, RuntimeRevision: c.Engine.RuntimeRevision, CacheType: c.Engine.CacheType, Capacity: c.Engine.Context, GPU: c.Engine.GPU, Device: c.Engine.Device, LogPath: filepath.Join(c.Home, "engine.log")}
	installed, err := catalog.Registry.List()
	if err != nil {
		return err
	}
	for _, bot := range installed {
		if bot.Adapter != "" {
			p.AdapterPaths = append(p.AdapterPaths, bot.Adapter)
		}
	}
	manager := provider.NewManager(p)
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if e := manager.Close(shutdown); e != nil {
			fmt.Fprintln(os.Stderr, "shutdown:", e)
		}
	}()
	r := &runner.Runner{ContextLimit: c.Engine.Context, Models: manager, Store: store, Plugins: plugins}
	if args[0] == "compare" {
		return compareCommand(ctx, c, r, args[1:])
	}
	if args[0] == "eval" {
		f := flags("eval")
		sizes := f.String("contexts", "2048,4096", "comma-separated context limits")
		output := f.String("output", "evaluation/baseline.json", "report path")
		if e = f.Parse(args[1:]); e != nil {
			return e
		}
		if f.NArg() != 0 {
			return errors.New("unexpected eval arguments")
		}
		var contexts []int
		for _, raw := range strings.Split(*sizes, ",") {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 512 || n > c.Engine.Context {
				return errors.New("evaluation context must be 512..engine.context_size")
			}
			contexts = append(contexts, n)
		}
		bots, e := reg.List()
		if e != nil {
			return e
		}
		if len(bots) == 0 {
			return errors.New("register bots before evaluation")
		}
		report, e := evaluation.Execute(ctx, r, bots, contexts)
		if e != nil {
			return e
		}
		report.Runtime = map[string]any{"model_id": c.Engine.ModelID, "engine_binary": c.Engine.Binary, "model_path": c.Engine.ModelPath, "device": c.Engine.Device, "process_starts": p.Starts, "artifact_lock": "artifacts/baseline.lock.json"}
		data, e := json.MarshalIndent(report, "", "  ")
		if e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Dir(*output), 0700); e != nil {
			return e
		}
		if e = os.WriteFile(*output, data, 0600); e != nil {
			return e
		}
		if e = write(map[string]any{"report": *output, "pass_rate": report.PassRate, "cases": len(report.Cases)}); e != nil {
			return e
		}
		if report.PassRate != 1 {
			return errors.New("baseline evaluation has failing fixtures")
		}
		return nil
	}
	if len(args) < 2 {
		return errors.New("run requires bot ID")
	}
	b, e := reg.Get(args[1])
	if e != nil {
		return e
	}
	f := flags("run")
	input := f.String("input", "", "task text (otherwise read stdin)")
	session := f.String("session", "", "resume session belonging to this bot/version")
	interactive := f.Bool("interactive", false, "read one task per line, preserving session")
	stream := f.Bool("stream", false, "stream text to stderr")
	asJSON := f.Bool("json", true, "emit JSON run record")
	override := settings(f)
	if e = f.Parse(args[2:]); e != nil {
		return e
	}
	if f.NArg() != 0 {
		return errors.New("unexpected run arguments")
	}
	var emit func(string)
	if *stream {
		emit = func(s string) { fmt.Fprint(os.Stderr, s) }
	}
	run := func(text string) error {
		result, e := r.Run(ctx, b, text, runner.Options{Session: *session, Settings: *override, Emit: emit})
		if *stream {
			fmt.Fprintln(os.Stderr)
		}
		if result.ID != "" {
			*session = result.Session
			if *asJSON {
				if outErr := write(result); outErr != nil {
					return outErr
				}
			} else {
				fmt.Fprintln(os.Stdout, result.Output)
			}
		}
		return e
	}
	if *interactive {
		if *input != "" {
			return errors.New("interactive cannot combine with input")
		}
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 4096), 1<<20)
		fmt.Fprintln(os.Stderr, "Enter a task per line; /quit exits.")
		for scanner.Scan() {
			text := scanner.Text()
			if text == "/quit" {
				break
			}
			if text == "" {
				continue
			}
			if e = run(text); e != nil {
				fmt.Fprintln(os.Stderr, e)
				if ctx.Err() != nil {
					return e
				}
			}
		}
		return scanner.Err()
	}
	if *input == "" {
		data, e := io.ReadAll(io.LimitReader(os.Stdin, (1<<20)+1))
		if e != nil {
			return e
		}
		*input = string(data)
	}
	return run(*input)
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if e := execute(ctx, os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		if ctx.Err() != nil {
			os.Exit(130)
		}
		if errors.Is(e, flag.ErrHelp) {
			return
		}
		os.Exit(1)
	}
}
