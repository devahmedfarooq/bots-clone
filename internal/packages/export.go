package packages

import (
	"archive/zip"
	"bots/internal/artifacts"
	"bots/internal/config"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Export writes a bounded portable snapshot. Shared weights are digest references.
func Export(b config.Bot, target string) error {
	b.ContextSources = append([]string{}, b.ContextSources...)
	files := map[string][]byte{}
	total := 0
	add := func(name string, data []byte) error {
		if !safeName(name) || len(data) > MaxFile || len(files) >= 126 {
			return errors.New("package payload exceeds limits")
		}
		if _, ok := files[name]; ok {
			return errors.New("duplicate payload")
		}
		total += len(data)
		if total > MaxArchive {
			return errors.New("package exceeds limit")
		}
		files[name] = data
		return nil
	}
	for i, root := range b.ContextSources {
		name := filepath.ToSlash(filepath.Join("context", string(rune('a'+i))))
		before := len(files)
		r, e := os.OpenRoot(root)
		if e != nil {
			return e
		}
		e = fs.WalkDir(r.FS(), ".", func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				return errors.New("symlink context cannot be exported")
			}
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				return errors.New("special context file")
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if info.Size() > MaxFile {
				return errors.New("context file too large")
			}
			data, e := fs.ReadFile(r.FS(), p)
			if e != nil {
				return e
			}
			return add(name+"/"+p, data)
		})
		r.Close()
		if e != nil {
			return e
		}
		if len(files) == before {
			if e := add(name+"/.keep", nil); e != nil {
				return e
			}
		}
		b.ContextSources[i] = name
	}
	if b.Evaluation != "" {
		info, e := os.Stat(b.Evaluation)
		if e != nil {
			return e
		}
		if info.Size() > MaxFile {
			return errors.New("evaluation file exceeds limit")
		}
		data, e := os.ReadFile(b.Evaluation)
		if e != nil {
			return e
		}
		if e = add("evaluation.json", data); e != nil {
			return e
		}
		b.Evaluation = "evaluation.json"
	}
	if b.Adapter != "" {
		a, e := artifacts.LoadAdapter(b.Adapter)
		if e != nil {
			return e
		}
		a.Artifact = "sha256:" + a.SHA256
		data, _ := json.MarshalIndent(a, "", "  ")
		if e = add("adapter.json", data); e != nil {
			return e
		}
		b.Adapter = "adapter.json"
	}
	data, _ := json.MarshalIndent(b, "", "  ")
	if e := add("bot.json", data); e != nil {
		return e
	}
	m := Manifest{Schema: 1, ID: b.ID, Version: b.Version, Files: map[string]string{}}
	for name, data := range files {
		sum := sha256.Sum256(data)
		m.Files[name] = hex.EncodeToString(sum[:])
	}
	data, _ = json.MarshalIndent(m, "", "  ")
	files["package.json"] = data
	f, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(target)
		}
	}()
	z := zip.NewWriter(f)
	for name, data := range files {
		w, e := z.Create(strings.ReplaceAll(name, "\\", "/"))
		if e != nil {
			return e
		}
		if _, e = w.Write(data); e != nil {
			return e
		}
	}
	if e = z.Close(); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	ok = true
	return nil
}
