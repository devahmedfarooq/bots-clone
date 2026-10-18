package evaluation

import (
	"bots/internal/config"
	"bots/internal/runner"
	"bots/internal/state"
	"context"
	"errors"
	"fmt"
	"time"
)

type SwitchCase struct {
	Cycle          int       `json:"cycle"`
	Target         string    `json:"target"`
	Run            state.Run `json:"run"`
	SwitchAndRunMS int64     `json:"switch_and_run_ms"`
	Repeatable     bool      `json:"repeatable"`
}

func Switches(ctx context.Context, r *runner.Runner, bots []config.Bot, cycles int) ([]SwitchCase, error) {
	if len(bots) != 2 || cycles < 2 || cycles > 20 {
		return nil, errors.New("switch acceptance requires two bots and 2..20 cycles")
	}
	for _, b := range bots {
		if b.Adapter == "" {
			return nil, errors.New("switch acceptance requires two adapters")
		}
	}
	base := bots[0]
	base.Adapter = ""
	base.Instructions = "Follow the user instruction."
	base.Tools = nil
	base.Retrieval = nil
	targets := append(append([]config.Bot{}, bots...), base)
	prior := map[string]string{}
	out := []SwitchCase{}
	failed := false
	for cycle := 0; cycle < cycles; cycle++ {
		for i, b := range targets {
			key := b.ID
			if i == 2 {
				key = "base"
			}
			started := time.Now()
			run, e := r.Run(ctx, b, "Return a short greeting.", runner.Options{})
			same := e == nil
			if old, ok := prior[key]; ok {
				same = same && old == run.Output
			}
			prior[key] = run.Output
			out = append(out, SwitchCase{cycle + 1, key, run, time.Since(started).Milliseconds(), same})
			if !same {
				failed = true
			}
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
		}
	}
	if failed {
		return out, fmt.Errorf("adapter switching failed or outputs were not repeatable; inspect runs")
	}
	return out, nil
}
