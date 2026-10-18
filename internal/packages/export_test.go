package packages

import (
	"bots/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestExportPreservesContextKeepFile(t *testing.T) {
	for _, empty := range []bool{false, true} {
		root := t.TempDir()
		if !empty {
			if e := os.WriteFile(filepath.Join(root, ".keep"), []byte("user context"), 0600); e != nil {
				t.Fatal(e)
			}
		}
		b := config.Bot{Schema: 1, ID: "context", Version: "1.0.0", Description: "test", Instructions: "test", BaseModel: "base", ContextSources: []string{root}, Budgets: config.Budgets{Tokens: 1000, Seconds: 10}}
		archive := filepath.Join(t.TempDir(), "export.zip")
		if e := Export(b, archive); e != nil {
			t.Fatal(e)
		}
		dir, cleanup, e := Extract(archive, t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		defer cleanup()
		loaded, e := config.Load(filepath.Join(dir, "bot.json"))
		if e != nil {
			t.Fatal(e)
		}
		data, e := os.ReadFile(filepath.Join(loaded.ContextSources[0], ".keep"))
		if e != nil {
			t.Fatal(e)
		}
		if !empty && string(data) != "user context" {
			t.Fatal("context content replaced")
		}
	}
}
