package workflow

import (
	"encoding/json"
	"errors"
	"fmt"

	"bots/internal/config"
	"bots/internal/registry"
)

type Definition struct {
	Schema int    `json:"schema_version"`
	ID     string `json:"id"`
	Steps  []Step `json:"steps"`
}
type Step struct {
	ID         string   `json:"id"`
	Bot        string   `json:"bot"`
	DependsOn  []string `json:"depends_on,omitempty"`
	Input      string   `json:"input,omitempty"`
	InputFrom  string   `json:"input_from,omitempty"`
	InputType  string   `json:"input_type"`
	OutputType string   `json:"output_type"`
	Retries    int      `json:"retries,omitempty"`
}

func Value(kind, text string) error {
	if len(text) == 0 || len(text) > 1<<20 {
		return errors.New("workflow value must be 1..1048576 bytes")
	}
	switch kind {
	case "text":
		return nil
	case "json":
		if json.Valid([]byte(text)) {
			return nil
		}
		return errors.New("workflow value must be valid JSON")
	default:
		return errors.New("workflow types must be text or json")
	}
}

func (d Definition) Validate(reg registry.Registry) ([]Step, error) {
	if d.Schema != 1 || !config.ValidID(d.ID) || len(d.Steps) < 1 || len(d.Steps) > 32 {
		return nil, errors.New("workflow requires schema 1, valid ID and 1..32 steps")
	}
	byID := map[string]Step{}
	for _, s := range d.Steps {
		if !config.ValidID(s.ID) || byID[s.ID].ID != "" {
			return nil, errors.New("invalid or duplicate step ID")
		}
		if s.InputType != "text" && s.InputType != "json" || s.OutputType != "text" && s.OutputType != "json" {
			return nil, errors.New("step types must be text or json")
		}
		if (s.Input == "") == (s.InputFrom == "") {
			return nil, errors.New("step requires exactly one of input or input_from")
		}
		if s.Input != "" {
			if err := Value(s.InputType, s.Input); err != nil {
				return nil, err
			}
		}
		b, err := reg.Get(s.Bot)
		if err != nil {
			return nil, fmt.Errorf("step %s: %w", s.ID, err)
		}
		if s.Retries < 0 || s.Retries > 2 || s.Retries > 0 && len(b.Tools) > 0 {
			return nil, errors.New("retries 0..2 allowed only for bots without tools")
		}
		byID[s.ID] = s
	}
	for _, s := range d.Steps {
		seen := map[string]bool{}
		for _, dep := range s.DependsOn {
			if byID[dep].ID == "" || dep == s.ID || seen[dep] {
				return nil, errors.New("invalid or duplicate dependency")
			}
			seen[dep] = true
		}
		if s.InputFrom != "" {
			source, ok := byID[s.InputFrom]
			if !ok || !seen[s.InputFrom] || source.OutputType != s.InputType {
				return nil, errors.New("input_from must reference a dependency with matching output type")
			}
		}
	}
	ordered := []Step{}
	done := map[string]bool{}
	for len(ordered) < len(d.Steps) {
		progress := false
		for _, s := range d.Steps {
			if done[s.ID] {
				continue
			}
			ready := true
			for _, dep := range s.DependsOn {
				if !done[dep] {
					ready = false
				}
			}
			if ready {
				ordered = append(ordered, s)
				done[s.ID] = true
				progress = true
			}
		}
		if !progress {
			return nil, errors.New("workflow contains a dependency cycle")
		}
	}
	return ordered, nil
}
