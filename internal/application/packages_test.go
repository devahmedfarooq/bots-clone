package application

import (
	"bots/internal/artifacts"
	"bots/internal/config"
	"bots/internal/packages"
	"bots/internal/registry"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func saveJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, data, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestAdapterPackageLifecycle(t *testing.T) {
	home := t.TempDir()
	weights := filepath.Join(home, "weights.gguf")
	raw := []byte{'G', 'G', 'U', 'F', 3, 0, 0, 0}
	os.WriteFile(weights, raw, 0600)
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	base := artifacts.Base{Schema: 1, Repository: "test/base", Revision: strings.Repeat("a", 40), Architecture: "qwen3", TokenizerRepository: "test/base", TokenizerRevision: strings.Repeat("a", 40), TokenizerSHA256: hash, SHA256: hash, Quantization: "Q8_0"}
	basePath := filepath.Join(home, "base.json")
	saveJSON(t, basePath, base)
	c := Catalog{Registry: registry.Registry{Dir: filepath.Join(home, "bots")}, Model: "base", BaseManifest: basePath, RuntimeRevision: "b11669", ArtifactDir: filepath.Join(home, "artifacts"), RuntimeBinary: weights}
	if _, e := (artifacts.Store{Dir: c.ArtifactDir}).Import(weights, hash); e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(home, "package")
	os.Mkdir(source, 0700)
	a := artifacts.Adapter{Schema: 1, ID: "special", Version: "1.0.0", Base: base, Runtime: "llama.cpp", RuntimeSHA256: hash, RuntimeRevision: "b11669", Format: "gguf-lora", Conversion: "llama.cpp/b11669/convert_lora_to_gguf.py", Source: "local:test", License: "CC0-1.0", SHA256: hash, Artifact: "sha256:" + hash, TargetModules: []string{"q_proj"}, TrainingMethod: "qlora"}
	saveJSON(t, filepath.Join(source, "adapter.json"), a)
	b := config.Bot{Schema: 1, ID: "third", Version: "1.0.0", Description: "test", Instructions: "test", BaseModel: "base", Adapter: "adapter.json", Budgets: config.Budgets{Tokens: 1000, Seconds: 10}}
	saveJSON(t, filepath.Join(source, "bot.json"), b)
	installed, e := c.Install(source)
	if e != nil {
		t.Fatal(e)
	}
	if installed.Adapter == "adapter.json" {
		t.Fatal("manifest not materialized")
	}
	b.Version = "1.1.0"
	b.Instructions = "updated"
	saveJSON(t, filepath.Join(source, "bot.json"), b)
	if _, e = c.Upgrade(source); e != nil {
		t.Fatal(e)
	}
	if _, e = c.Upgrade(source); e == nil {
		t.Fatal("same version overwritten")
	}
	rolled, e := c.Rollback("third", "1.0.0")
	if e != nil || rolled.Instructions != "test" {
		t.Fatal(rolled, e)
	}
	archive := filepath.Join(home, "export.zip")
	if e = packages.Export(rolled, archive); e != nil {
		t.Fatal(e)
	}
	if e = c.Registry.Remove("third"); e != nil {
		t.Fatal(e)
	}
	if _, e = (artifacts.Store{Dir: c.ArtifactDir}).Resolve(hash); e != nil {
		t.Fatal("uninstall deleted shared weights", e)
	}
	imported, e := c.Import(archive, filepath.Join(home, "imports"))
	if e != nil || imported.ID != "third" {
		t.Fatal(imported, e)
	}
	wrong := base
	wrong.Revision = strings.Repeat("b", 40)
	saveJSON(t, basePath, wrong)
	if c.Check(imported) == nil {
		t.Fatal("base mismatch accepted")
	}
}
