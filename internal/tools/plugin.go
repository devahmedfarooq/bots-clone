package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"bots/internal/config"
	"bots/internal/provider"
	"bots/plugin"
)

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}
type PluginSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
	Additional bool                `json:"additionalProperties"`
}
type PluginManifest struct {
	Version     int          `json:"protocol_version"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Executable  string       `json:"executable"`
	Parameters  PluginSchema `json:"parameters"`
}
type PluginTool struct{ Manifest PluginManifest }

func LoadPlugins(paths []string) ([]Tool, error) {
	out := []Tool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if len(data) > 16384 {
			return nil, errors.New("plugin manifest exceeds 16 KiB")
		}
		var m PluginManifest
		if err = config.Decode(data, &m); err != nil {
			return nil, err
		}
		if m.Version != plugin.Version || !config.ValidID(m.Name) || m.Description == "" || m.Executable == "" || m.Parameters.Type != "object" || m.Parameters.Additional {
			return nil, errors.New("invalid plugin manifest or unsupported protocol/schema")
		}
		for key, p := range m.Parameters.Properties {
			if key == "" || p.Type != "string" && p.Type != "boolean" && p.Type != "number" && p.Type != "integer" {
				return nil, errors.New("plugin properties support string, boolean, number, integer")
			}
		}
		seen := map[string]bool{}
		for _, key := range m.Parameters.Required {
			if _, ok := m.Parameters.Properties[key]; !ok || seen[key] {
				return nil, errors.New("invalid required plugin property")
			}
			seen[key] = true
		}
		if !filepath.IsAbs(m.Executable) {
			m.Executable = filepath.Join(filepath.Dir(path), m.Executable)
		}
		m.Executable, err = filepath.Abs(m.Executable)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(m.Executable)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("plugin executable must be a regular file")
		}
		out = append(out, &PluginTool{Manifest: m})
	}
	return out, nil
}
func (p *PluginTool) Definition() provider.ToolDefinition {
	schema, _ := json.Marshal(p.Manifest.Parameters)
	return provider.ToolDefinition{Name: p.Manifest.Name, Description: p.Manifest.Description, Parameters: schema}
}
func (p *PluginTool) Validate(raw json.RawMessage) error {
	if len(raw) > plugin.Limit {
		return errors.New("plugin arguments too large")
	}
	var args map[string]any
	if err := config.Decode(raw, &args); err != nil {
		return err
	}
	if args == nil {
		return errors.New("plugin arguments must be object")
	}
	for _, key := range p.Manifest.Parameters.Required {
		if _, ok := args[key]; !ok {
			return fmt.Errorf("missing %s", key)
		}
	}
	for key, value := range args {
		property, ok := p.Manifest.Parameters.Properties[key]
		if !ok {
			return fmt.Errorf("unknown argument %s", key)
		}
		valid := false
		switch property.Type {
		case "string":
			_, valid = value.(string)
		case "boolean":
			_, valid = value.(bool)
		case "number":
			_, valid = value.(float64)
		case "integer":
			n, ok := value.(float64)
			valid = ok && math.Trunc(n) == n
		}
		if !valid {
			return fmt.Errorf("invalid type for %s", key)
		}
	}
	return nil
}

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if b.Len()+len(data) > plugin.Limit {
		return 0, errors.New("plugin output exceeds 64 KiB")
	}
	return b.Buffer.Write(data)
}
func (p *PluginTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	if err := p.Validate(raw); err != nil {
		return "", err
	}
	data, err := json.Marshal(plugin.Request{Version: plugin.Version, Arguments: raw})
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, p.Manifest.Executable)
	cmd.Env = []string{}
	for _, key := range []string{"SystemRoot", "WINDIR", "TEMP", "TMP"} {
		if value := os.Getenv(key); value != "" {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	cmd.Stdin = bytes.NewReader(data)
	var output boundedBuffer
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	if err = cmd.Run(); err != nil {
		return "", fmt.Errorf("plugin execution: %w", err)
	}
	var response plugin.Response
	if err = config.Decode(output.Bytes(), &response); err != nil {
		return "", err
	}
	if response.Version != plugin.Version {
		return "", errors.New("unsupported plugin response version")
	}
	if response.Error != "" {
		return "", errors.New(response.Error)
	}
	return response.Output, nil
}

func WithPlugins(paths []string, extra []Tool) (*Registry, func(), error) {
	r, close, err := Files(paths)
	if err != nil {
		return nil, nil, err
	}
	for _, t := range extra {
		if err = r.Register(t); err != nil {
			close()
			return nil, nil, err
		}
	}
	return r, close, nil
}
