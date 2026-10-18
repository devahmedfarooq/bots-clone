package artifacts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Base struct {
	Schema              int    `json:"schema_version"`
	Repository          string `json:"repository"`
	Revision            string `json:"revision"`
	Architecture        string `json:"architecture"`
	TokenizerRepository string `json:"tokenizer_repository"`
	TokenizerRevision   string `json:"tokenizer_revision"`
	TokenizerSHA256     string `json:"tokenizer_sha256"`
	SHA256              string `json:"sha256"`
	Quantization        string `json:"quantization"`
}
type Adapter struct {
	Schema          int      `json:"schema_version"`
	ID              string   `json:"id"`
	Version         string   `json:"version"`
	Base            Base     `json:"base"`
	Runtime         string   `json:"runtime"`
	RuntimeSHA256   string   `json:"runtime_sha256"`
	RuntimeRevision string   `json:"runtime_revision"`
	Format          string   `json:"format"`
	Conversion      string   `json:"conversion"`
	Source          string   `json:"source"`
	License         string   `json:"license"`
	SHA256          string   `json:"sha256"`
	Artifact        string   `json:"artifact"`
	TargetModules   []string `json:"target_modules"`
	TrainingMethod  string   `json:"training_method"`
}

var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)
var revision = regexp.MustCompile(`^[a-f0-9]{40}$`)
var id = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var version = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func read(path string, target any) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	if e = d.Decode(target); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("expected one manifest object")
	}
	return nil
}
func (b Base) Validate() error {
	if b.Schema != 1 || b.Repository == "" || !revision.MatchString(b.Revision) || b.Architecture == "" || b.TokenizerRepository == "" || !revision.MatchString(b.TokenizerRevision) || !digest.MatchString(b.TokenizerSHA256) || !digest.MatchString(b.SHA256) || b.Quantization == "" {
		return errors.New("base requires exact source/tokenizer revisions, tokenizer and GGUF SHA256, architecture and quantization")
	}
	return nil
}
func LoadBase(path string) (Base, error) {
	var b Base
	e := read(path, &b)
	if e == nil {
		e = b.Validate()
	}
	return b, e
}
func LoadAdapter(path string) (Adapter, error) {
	var a Adapter
	e := read(path, &a)
	if e == nil {
		e = a.Validate()
	}
	if e == nil && !strings.HasPrefix(a.Artifact, "sha256:") && !filepath.IsAbs(a.Artifact) {
		a.Artifact = filepath.Join(filepath.Dir(path), a.Artifact)
	}
	return a, e
}
func (a Adapter) Validate() error {
	if a.Schema != 1 || !id.MatchString(a.ID) || !version.MatchString(a.Version) || a.Source == "" || a.License == "" || !digest.MatchString(a.SHA256) || a.Artifact == "" || a.TrainingMethod != "qlora" {
		return errors.New("adapter requires schema 1, ID/version, QLoRA provenance, license, artifact and SHA256")
	}
	if e := a.Base.Validate(); e != nil {
		return e
	}
	if a.Runtime != "llama.cpp" || !digest.MatchString(a.RuntimeSHA256) || a.RuntimeRevision == "" || a.Format != "gguf-lora" || a.Conversion != "llama.cpp/"+a.RuntimeRevision+"/convert_lora_to_gguf.py" {
		return errors.New("supported deployment: gguf-lora converted with a pinned llama.cpp revision; convert PEFT using convert_lora_to_gguf.py")
	}
	if len(a.TargetModules) == 0 {
		return errors.New("target_modules required")
	}
	allowed := map[string]bool{"q_proj": true, "k_proj": true, "v_proj": true, "o_proj": true, "gate_proj": true, "up_proj": true, "down_proj": true}
	seen := map[string]bool{}
	for _, m := range a.TargetModules {
		if !allowed[m] || seen[m] {
			return fmt.Errorf("unsupported or duplicate target module %q", m)
		}
		seen[m] = true
	}
	return nil
}
func (a Adapter) Compatible(base Base, runtime string) error {
	if e := a.Validate(); e != nil {
		return e
	}
	if e := base.Validate(); e != nil {
		return e
	}
	if a.Base != base {
		return errors.New("adapter exact base/tokenizer/quantization identity mismatch")
	}
	if a.RuntimeRevision != runtime {
		return errors.New("adapter runtime revision mismatch")
	}
	return nil
}
func Verify(path, expected string) error {
	if !digest.MatchString(expected) {
		return errors.New("invalid SHA256")
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return errors.New("artifact must be a regular file")
	}
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return e
	}
	if hex.EncodeToString(h.Sum(nil)) != expected {
		return errors.New("artifact checksum mismatch")
	}
	return nil
}
func (a Adapter) Verify() error {
	if e := Verify(a.Artifact, a.SHA256); e != nil {
		return e
	}
	f, e := os.Open(a.Artifact)
	if e != nil {
		return e
	}
	defer f.Close()
	header := make([]byte, 8)
	if _, e = io.ReadFull(f, header); e != nil {
		return e
	}
	if !bytes.Equal(header, []byte{'G', 'G', 'U', 'F', 3, 0, 0, 0}) {
		return errors.New("adapter must be GGUF version 3")
	}
	return nil
}
