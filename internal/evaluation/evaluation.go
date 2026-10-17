package evaluation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"time"

	"bots/internal/config"
	"bots/internal/diagnostics"
	"bots/internal/runner"
	"bots/internal/state"
)

type Fixture struct {
	Name         string          `json:"name"`
	Input        string          `json:"input"`
	ExpectedJSON json.RawMessage `json:"expected_json,omitempty"`
	Contains     []string        `json:"contains,omitempty"`
	MinToolCalls int             `json:"min_tool_calls,omitempty"`
}
type Case struct {
	Bot     string    `json:"bot"`
	Fixture string    `json:"fixture"`
	Context int       `json:"context_size"`
	Passed  bool      `json:"passed"`
	Run     state.Run `json:"run"`
}
type Report struct {
	At            time.Time            `json:"at"`
	Hardware      diagnostics.Report   `json:"hardware"`
	Cases         []Case               `json:"cases"`
	PassRate      float64              `json:"pass_rate"`
	MemorySamples []diagnostics.Sample `json:"memory_samples"`
	PeakVRAMMiB   uint64               `json:"peak_device_vram_mib"`
	PeakRAMBytes  uint64               `json:"peak_system_used_ram_bytes"`
	Notes         string               `json:"notes"`
	Runtime       map[string]any       `json:"runtime"`
}

func Execute(ctx context.Context, r *runner.Runner, bots []config.Bot, contexts []int) (report Report, err error) {
	report = Report{At: time.Now().UTC(), Hardware: diagnostics.Collect(), Notes: "Memory is sampled device-wide VRAM and system-wide used RAM, including other applications; not isolated model allocations. First case at each context size includes cold load; later cases at that size reuse weights. Synthetic fixtures are a baseline, not specialization evidence."}
	var samples []diagnostics.Sample
	var peakVRAM, peakRAM uint64
	sample := func() {
		s := diagnostics.Measure()
		samples = append(samples, s)
		peakVRAM = max(peakVRAM, s.VRAMMiB)
		peakRAM = max(peakRAM, s.RAMBytes)
	}
	sample()
	monitorCtx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				sample()
			}
		}
	}()
	defer func() {
		stop()
		<-done
		sample()
		report.MemorySamples = samples
		report.PeakVRAMMiB = peakVRAM
		report.PeakRAMBytes = peakRAM
	}()
	passed := 0
	for _, size := range contexts {
		for _, b := range bots {
			data, e := os.ReadFile(b.Evaluation)
			if e != nil {
				return report, e
			}
			var fixtures []Fixture
			if e = config.Decode(data, &fixtures); e != nil {
				return report, e
			}
			if len(fixtures) == 0 {
				return report, errors.New("empty fixture suite")
			}
			for _, f := range fixtures {
				if e := ctx.Err(); e != nil {
					return report, e
				}
				run, err := r.Run(ctx, b, f.Input, runner.Options{Settings: config.Settings{ContextSize: size}})
				ok := err == nil
				if len(f.ExpectedJSON) > 0 {
					var expected, actual any
					if json.Unmarshal(f.ExpectedJSON, &expected) != nil || json.Unmarshal([]byte(run.Output), &actual) != nil || !reflect.DeepEqual(expected, actual) {
						ok = false
					}
				}
				for _, term := range f.Contains {
					if !strings.Contains(strings.ToLower(run.Output), strings.ToLower(term)) {
						ok = false
					}
				}
				if f.MinToolCalls > 0 {
					count, e := r.Store.ToolEvents(run.ID)
					if e != nil || count < f.MinToolCalls {
						ok = false
					}
				}
				if ok {
					passed++
				}
				report.Cases = append(report.Cases, Case{b.ID, f.Name, size, ok, run})
			}
		}
	}
	if len(report.Cases) > 0 {
		report.PassRate = float64(passed) / float64(len(report.Cases))
	}
	return report, nil
}
