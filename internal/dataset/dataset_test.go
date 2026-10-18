package dataset

import (
	"bots/internal/provider"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func record(id, split, input string) Record {
	return Record{ID: id, Task: "extract", Group: id, Split: split, Source: Source{URI: "local:synthetic", License: "CC0-1.0", Revision: "v1", Synthetic: true}, Messages: []provider.Message{{Role: "user", Content: input}, {Role: "assistant", Content: `{"label":"ok"}`}}, OutputSchema: map[string]string{"label": "string"}}
}
func dataFile(t *testing.T, records ...Record) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "data.jsonl")
	f, _ := os.Create(p)
	for _, r := range records {
		json.NewEncoder(f).Encode(r)
	}
	f.Close()
	return p
}
func TestDatasetRejections(t *testing.T) {
	held := dataFile(t, record("held", "test", "held out"))
	good := record("a", "train", "training example")
	if _, _, e := Validate(dataFile(t, good), held); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []Record{record("b", "test", "TRAINING  example"), record("b", "train", "held out"), func() Record { r := record("b", "test", "different input"); r.Group = "a"; return r }(), func() Record { r := record("b", "train", "api_key=abcdefghijklmnopqrstuvwx"); return r }(), func() Record {
		r := record("b", "train", "different input")
		r.Messages[1].Content = `{"label":42}`
		return r
	}()} {
		_, report, e := Validate(dataFile(t, good, bad), held)
		if e == nil || len(report.Rejected) == 0 || report.Rejected[0].Line != 2 {
			t.Fatal(report, e)
		}
	}
	if validate(record("a", "train", `{"password":"sensitive-fixture-value"}`)) == nil {
		t.Fatal("JSON credentials accepted")
	}
	if validate(record("a", "invalid", "x")) == nil {
		t.Fatal("bad split accepted")
	}
	if !secret.MatchString("-----BEGIN RSA PRIVATE KEY-----") {
		t.Fatal("private key detection missing")
	}
	_ = strings.TrimSpace
}
