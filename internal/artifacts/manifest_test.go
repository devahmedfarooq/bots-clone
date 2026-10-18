package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompatibilityAndStorage(t *testing.T) {
	dir := t.TempDir()
	data := []byte{'G', 'G', 'U', 'F', 3, 0, 0, 0}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	source := filepath.Join(dir, "source.gguf")
	os.WriteFile(source, data, 0600)
	b := Base{Schema: 1, Repository: "Qwen/base", Revision: strings.Repeat("a", 40), Architecture: "qwen3", TokenizerRepository: "Qwen/base", TokenizerRevision: strings.Repeat("a", 40), TokenizerSHA256: hash, SHA256: hash, Quantization: "Q8_0"}
	a := Adapter{Schema: 1, ID: "test", Version: "1.0.0", Base: b, Runtime: "llama.cpp", RuntimeRevision: "b11669", RuntimeSHA256: hash, Format: "gguf-lora", Conversion: "llama.cpp/b11669/convert_lora_to_gguf.py", Source: "local:dataset-v1", License: "CC0-1.0", SHA256: hash, Artifact: source, TargetModules: []string{"q_proj"}, TrainingMethod: "qlora"}
	if e := a.Compatible(b, "b11669"); e != nil {
		t.Fatal(e)
	}
	if e := a.Verify(); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*Base){func(b *Base) { b.Revision = strings.Repeat("b", 40) }, func(b *Base) { b.TokenizerSHA256 = strings.Repeat("0", 64) }, func(b *Base) { b.TokenizerRevision = strings.Repeat("b", 40) }, func(b *Base) { b.Quantization = "Q4_0" }, func(b *Base) { b.SHA256 = strings.Repeat("0", 64) }} {
		other := b
		change(&other)
		if a.Compatible(other, "b11669") == nil {
			t.Fatal("mismatch accepted")
		}
	}
	if a.Compatible(b, "wrong") == nil {
		t.Fatal("runtime mismatch accepted")
	}
	store := Store{Dir: filepath.Join(dir, "store")}
	p, e := store.Import(source, hash)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = store.Import(source, hash); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(p, []byte("corrupt"), 0600)
	if _, e = store.Resolve(hash); e == nil {
		t.Fatal("corrupt artifact accepted")
	}
	if _, e = store.Path("../escape"); e == nil {
		t.Fatal("unsafe hash accepted")
	}
	a.Artifact = "sha256:" + hash
	raw, _ := json.Marshal(a)
	manifest := filepath.Join(dir, "adapter.json")
	os.WriteFile(manifest, raw, 0600)
	loaded, e := LoadAdapter(manifest)
	if e != nil || loaded.Artifact != a.Artifact {
		t.Fatal(loaded, e)
	}
}
