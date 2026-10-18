package packages

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bots/internal/config"
)

func fixture(t *testing.T, change func(map[string][]byte, *Manifest)) string {
	t.Helper()
	bot := config.Bot{Schema: 1, ID: "test", Version: "1.0.0", Description: "test", Instructions: "test", BaseModel: "base", Budgets: config.Budgets{Tokens: 100, Seconds: 5}}
	data, _ := json.Marshal(bot)
	sum := sha256.Sum256(data)
	files := map[string][]byte{"bot.json": data}
	m := Manifest{Schema: 1, ID: bot.ID, Version: bot.Version, Files: map[string]string{"bot.json": hex.EncodeToString(sum[:])}}
	if change != nil {
		change(files, &m)
	}
	data, _ = json.Marshal(m)
	files["package.json"] = data
	path := filepath.Join(t.TempDir(), "package.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for name, data := range files {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}
func TestImportValidation(t *testing.T) {
	path := fixture(t, nil)
	dir, cleanup, err := Extract(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err = config.Load(filepath.Join(dir, "bot.json")); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(map[string][]byte, *Manifest){
		func(f map[string][]byte, m *Manifest) { f["../escape.txt"] = []byte("bad") },
		func(f map[string][]byte, m *Manifest) { m.Files["bot.json"] = strings.Repeat("0", 64) },
		func(f map[string][]byte, m *Manifest) { f["BOT.JSON"] = f["bot.json"] },
		func(f map[string][]byte, m *Manifest) { f["CON.txt"] = []byte("bad") },
		func(f map[string][]byte, m *Manifest) { f["large.txt"] = []byte(strings.Repeat("x", MaxFile+1)) },
		func(f map[string][]byte, m *Manifest) { f["unlisted.txt"] = []byte("bad") },
	} {
		if _, _, err = Extract(fixture(t, change), t.TempDir()); err == nil {
			t.Fatal("malicious archive accepted")
		}
	}
}
