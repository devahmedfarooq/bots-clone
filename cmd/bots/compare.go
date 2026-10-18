package main

import (
	"bots/internal/config"
	"bots/internal/evaluation"
	"bots/internal/provider"
	"bots/internal/registry"
	"bots/internal/runner"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func compareCommand(ctx context.Context, c appConfig, r *runner.Runner, args []string) error {
	f := flags("compare")
	ids := f.String("bots", "", "comma-separated installed adapter bot IDs")
	sizes := f.String("contexts", "2048", "context sizes")
	cycles := f.Int("switch-cycles", 0, "repeat A/B/base instead of quality comparison")
	output := f.String("output", "evaluation/phase2-comparison.json", "report path")
	if e := f.Parse(args); e != nil {
		return e
	}
	if *ids == "" || f.NArg() != 0 {
		return errors.New("compare requires -bots ID,ID")
	}
	var bots []config.Bot
	reg := registry.Registry{Dir: filepath.Join(c.Home, "bots")}
	for _, id := range strings.Split(*ids, ",") {
		b, e := reg.Get(id)
		if e != nil {
			return e
		}
		bots = append(bots, b)
	}
	var contexts []int
	for _, s := range strings.Split(*sizes, ",") {
		n, e := strconv.Atoi(s)
		if e != nil || n < 512 || n > c.Engine.Context {
			return errors.New("invalid comparison context")
		}
		contexts = append(contexts, n)
	}
	if *cycles > 0 {
		cases, e := evaluation.Switches(ctx, r, bots, *cycles)
		starts := 0
		if p, ok := r.Models.Provider.(*provider.Llama); ok {
			starts = p.Starts
		}
		data, _ := json.MarshalIndent(map[string]any{"cases": cases, "process_starts": starts, "error": fmt.Sprint(e)}, "", "  ")
		if save := os.WriteFile(*output, data, 0600); save != nil {
			return save
		}
		return e
	}
	report, e := evaluation.Compare(ctx, r, bots, contexts)
	if e != nil {
		return e
	}
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
	return write(map[string]string{"report": *output, "status": "experimental; inspect all three arms"})
}
