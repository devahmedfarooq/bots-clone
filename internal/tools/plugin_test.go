package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"bots/internal/provider"
)

func TestPluginSubprocessAndGrants(t *testing.T) {
	path := filepath.Join(t.TempDir(), "echo")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", path, "../../cmd/plugin-echo")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(string(output), err)
	}
	m := PluginManifest{Version: 1, Name: "echo", Description: "test", Executable: path, Parameters: PluginSchema{Type: "object", Properties: map[string]Property{"text": {Type: "string"}}, Required: []string{"text"}}}
	manifest := filepath.Join(filepath.Dir(path), "manifest.json")
	data, _ := json.Marshal(m)
	os.WriteFile(manifest, data, 0600)
	plugins, err := LoadPlugins([]string{manifest})
	if err != nil {
		t.Fatal(err)
	}
	r, close, err := WithPlugins(nil, plugins)
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	call := provider.Call{Function: provider.Function{Name: "echo", Arguments: `{"text":"hello"}`}}
	if result := r.Call(context.Background(), nil, call); result.Error == nil || result.Error.Code != "permission_denied" {
		t.Fatal(result)
	}
	if result := r.Call(context.Background(), []string{"echo"}, call); result.Error != nil || result.Output != "hello" {
		t.Fatal(result)
	}
	call.Function.Arguments = `{"text":42}`
	if result := r.Call(context.Background(), []string{"echo"}, call); result.Error == nil || result.Error.Code != "invalid_arguments" {
		t.Fatal(result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := plugins[0].Execute(ctx, json.RawMessage(`{"text":"hello"}`)); err == nil {
		t.Fatal("cancelled plugin executed")
	}
}
