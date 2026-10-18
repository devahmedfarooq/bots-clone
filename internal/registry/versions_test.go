package registry

import (
	"bots/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestFailedUpgradePreservesRegistration(t *testing.T) {
	r := Registry{Dir: filepath.Join(t.TempDir(), "bots")}
	b := config.Bot{Schema: 1, ID: "test", Version: "1.0.0", Description: "test", Instructions: "one", BaseModel: "base", Budgets: config.Budgets{Tokens: 1000, Seconds: 10}}
	if _, e := r.RegisterBot(b); e != nil {
		t.Fatal(e)
	}
	b.Version = "1.1.0"
	b.Instructions = "two"
	if _, e := r.Replace(b); e != nil {
		t.Fatal(e)
	}
	old, e := r.Previous("test", "1.0.0")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.Replace(old); e != nil {
		t.Fatal(e)
	}
	b.Instructions = "different contents"
	if _, e = r.Replace(b); e == nil {
		t.Fatal("mutable version allowed")
	}
	current, e := r.Get("test")
	if e != nil || current.Version != "1.0.0" {
		t.Fatal(current, e)
	}
	if _, e = r.Previous("test", "../escape"); e == nil {
		t.Fatal("unsafe version")
	}
	if _, e = os.Stat(filepath.Join(r.Dir, "versions", "test", "1.1.0.json")); e != nil {
		t.Fatal(e)
	}
}
