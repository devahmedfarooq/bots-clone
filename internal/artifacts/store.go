package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type Store struct{ Dir string }

func (s Store) Path(sum string) (string, error) {
	if !digest.MatchString(sum) {
		return "", errors.New("invalid artifact digest")
	}
	return filepath.Join(s.Dir, sum+".gguf"), nil
}
func (s Store) Resolve(sum string) (string, error) {
	p, e := s.Path(sum)
	if e == nil {
		e = Verify(p, sum)
	}
	return p, e
}
func (s Store) Import(source, sum string) (string, error) {
	target, e := s.Path(sum)
	if e != nil {
		return "", e
	}
	if _, e = os.Stat(target); e == nil {
		return s.Resolve(sum)
	} else if !os.IsNotExist(e) {
		return "", e
	}
	if e = os.MkdirAll(s.Dir, 0700); e != nil {
		return "", e
	}
	in, e := os.Open(source)
	if e != nil {
		return "", e
	}
	defer in.Close()
	info, e := in.Stat()
	if e != nil {
		return "", e
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("artifact must be regular")
	}
	out, e := os.CreateTemp(s.Dir, ".import-")
	if e != nil {
		return "", e
	}
	defer os.Remove(out.Name())
	h := sha256.New()
	_, e = io.Copy(io.MultiWriter(out, h), in)
	if e != nil {
		out.Close()
		return "", e
	}
	if e = out.Sync(); e != nil {
		out.Close()
		return "", e
	}
	if e = out.Close(); e != nil {
		return "", e
	}
	if hex.EncodeToString(h.Sum(nil)) != sum {
		return "", errors.New("artifact checksum mismatch")
	}
	// A hard link publishes without replacing an existing content-addressed artifact.
	if e = os.Link(out.Name(), target); e != nil {
		if _, check := s.Resolve(sum); check != nil {
			return "", e
		}
	}
	return s.Resolve(sum)
}
