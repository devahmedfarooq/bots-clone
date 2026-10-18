package packages

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bots/internal/config"
	"bots/internal/state"
)

const MaxArchive = 16 << 20
const MaxFile = 1 << 20

type Manifest struct {
	Schema  int               `json:"schema_version"`
	ID      string            `json:"id"`
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

func safeName(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\:") || !filepath.IsLocal(filepath.FromSlash(name)) {
		return false
	}
	for _, part := range strings.Split(strings.TrimSuffix(name, "/"), "/") {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, ". ") != part {
			return false
		}
		for _, c := range part {
			if c < 32 {
				return false
			}
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
			return false
		}
	}
	return true
}

func Extract(path, home string) (dir string, cleanup func(), err error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, err
	}
	if info.Size() > MaxArchive {
		return "", nil, errors.New("archive exceeds 16 MiB")
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return "", nil, err
	}
	defer z.Close()
	if len(z.File) > 128 {
		return "", nil, errors.New("archive exceeds 128 entries")
	}
	files := map[string][]byte{}
	seen := map[string]bool{}
	total := 0
	for _, file := range z.File {
		name := file.Name
		key := strings.ToLower(strings.TrimSuffix(name, "/"))
		if !safeName(name) || seen[key] || file.Mode()&os.ModeSymlink != 0 || !file.FileInfo().IsDir() && !file.Mode().IsRegular() {
			return "", nil, errors.New("unsafe, duplicate or special archive path")
		}
		seen[key] = true
		if file.FileInfo().IsDir() {
			continue
		}
		if file.UncompressedSize64 > MaxFile {
			return "", nil, errors.New("package file exceeds 1 MiB")
		}
		r, err := file.Open()
		if err != nil {
			return "", nil, err
		}
		data, err := io.ReadAll(io.LimitReader(r, MaxFile+1))
		closeErr := r.Close()
		if err != nil {
			return "", nil, err
		}
		if closeErr != nil {
			return "", nil, closeErr
		}
		if len(data) > MaxFile {
			return "", nil, errors.New("package file exceeds 1 MiB")
		}
		total += len(data)
		if total > MaxArchive {
			return "", nil, errors.New("unpacked package exceeds 16 MiB")
		}
		files[name] = data
	}
	var manifest Manifest
	if err = config.Decode(files["package.json"], &manifest); err != nil {
		return "", nil, err
	}
	if manifest.Schema != 1 || !config.ValidID(manifest.ID) || len(manifest.Files) != len(files)-1 {
		return "", nil, errors.New("invalid package manifest or unlisted files")
	}
	for name, data := range files {
		if name == "package.json" {
			continue
		}
		expected, ok := manifest.Files[name]
		sum := sha256.Sum256(data)
		if !ok || expected != hex.EncodeToString(sum[:]) {
			return "", nil, errors.New("package checksum mismatch")
		}
	}
	var bot config.Bot
	if err = config.Decode(files["bot.json"], &bot); err != nil {
		return "", nil, err
	}
	if err = bot.Validate(); err != nil {
		return "", nil, err
	}
	if bot.ID != manifest.ID || bot.Version != manifest.Version {
		return "", nil, errors.New("package and bot identity mismatch")
	}
	for _, path := range append(append([]string{}, bot.ContextSources...), bot.Evaluation) {
		if path != "" && !safeName(path) {
			return "", nil, errors.New("archive references must remain inside the package")
		}
	}
	if err = os.MkdirAll(home, 0700); err != nil {
		return "", nil, err
	}
	dir = filepath.Join(home, manifest.ID+"-"+manifest.Version+"-"+state.ID())
	if err = os.Mkdir(dir, 0700); err != nil {
		return "", nil, err
	}
	extractionDir := dir
	cleanup = func() { _ = os.RemoveAll(extractionDir) }
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	for name, data := range files {
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return "", nil, err
		}
		if err = os.WriteFile(target, data, 0600); err != nil {
			return "", nil, err
		}
	}
	return dir, cleanup, nil
}
