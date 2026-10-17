package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"bots/internal/config"
	"bots/internal/provider"
)

type FileArgs struct {
	Root    int    `json:"root"`
	Path    string `json:"path"`
	Query   string `json:"query,omitempty"`
	Content string `json:"content,omitempty"`
}
type SearchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}
type FileTool struct {
	Kind  string
	Roots []*os.Root
}

func Files(paths []string) (*Registry, func(), error) {
	r := New()
	roots := []*os.Root{}
	close := func() {
		for _, root := range roots {
			root.Close()
		}
	}
	for _, p := range paths {
		root, e := os.OpenRoot(p)
		if e != nil {
			close()
			return nil, nil, e
		}
		roots = append(roots, root)
	}
	for _, kind := range []string{"file-read", "file-search", "file-write"} {
		if e := r.Register(&FileTool{kind, roots}); e != nil {
			close()
			return nil, nil, e
		}
	}
	return r, close, nil
}
func (t *FileTool) Definition() provider.ToolDefinition {
	properties := map[string]any{"root": map[string]any{"type": "integer", "minimum": 0}, "path": map[string]any{"type": "string"}}
	required := []string{"root", "path"}
	description := "Read a UTF-8 file under an allowed root index. Use only relative paths."
	if t.Kind == "file-search" {
		properties["query"] = map[string]string{"type": "string"}
		required = append(required, "query")
		description = "Search literal text under relative path (. means root). Returns a JSON array of path, line, text records. To read full context, pass the EXACT path field from a match to file-read."
	}
	if t.Kind == "file-write" {
		properties["content"] = map[string]string{"type": "string"}
		required = append(required, "content")
		description = "Create a new UTF-8 file under a root. Never overwrite existing files; parent must exist."
	}
	schema, _ := json.Marshal(map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false})
	return provider.ToolDefinition{Name: t.Kind, Description: description, Parameters: schema}
}
func (t *FileTool) args(raw json.RawMessage) (FileArgs, error) {
	var a FileArgs
	if e := config.Decode(raw, &a); e != nil {
		return a, e
	}
	var fields map[string]any
	if e := json.Unmarshal(raw, &fields); e != nil {
		return a, e
	}
	for _, required := range []string{"root", "path"} {
		if fields[required] == nil {
			return a, fmt.Errorf("%s is required", required)
		}
	}
	if a.Root < 0 || a.Root >= len(t.Roots) {
		return a, errors.New("root index is not granted")
	}
	if !filepath.IsLocal(a.Path) || strings.Contains(a.Path, ":") {
		return a, errors.New("path must stay relative to root")
	}
	if t.Kind == "file-search" && (a.Query == "" || len(a.Query) > 1024) {
		return a, errors.New("query must be 1..1024 bytes")
	}
	if t.Kind == "file-write" {
		if fields["content"] == nil || len(a.Content) > 65536 {
			return a, errors.New("content required, maximum 65536 bytes")
		}
	}
	if t.Kind != "file-search" && a.Query != "" || t.Kind != "file-write" && a.Content != "" {
		return a, errors.New("arguments do not match tool")
	}
	return a, nil
}
func (t *FileTool) Validate(raw json.RawMessage) error { _, e := t.args(raw); return e }
func (t *FileTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	a, e := t.args(raw)
	if e != nil {
		return "", e
	}
	if e = ctx.Err(); e != nil {
		return "", e
	}
	root := t.Roots[a.Root]
	switch t.Kind {
	case "file-read":
		f, e := root.Open(a.Path)
		if e != nil {
			return "", e
		}
		defer f.Close()
		info, e := f.Stat()
		if e != nil {
			return "", e
		}
		if !info.Mode().IsRegular() {
			return "", errors.New("regular file required")
		}
		b, e := io.ReadAll(io.LimitReader(f, 65537))
		if len(b) > 65536 {
			return "", errors.New("file exceeds 65536 bytes")
		}
		return string(b), e
	case "file-write":
		if e = ctx.Err(); e != nil {
			return "", e
		}
		f, e := root.OpenFile(a.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return "", e
		}
		_, e = f.WriteString(a.Content)
		closeErr := f.Close()
		if e != nil {
			return "", e
		}
		return "created " + a.Path, closeErr
	case "file-search":
		matches := []SearchMatch{}
		outputBytes := 0
		visited := 0
		e = fs.WalkDir(root.FS(), filepath.ToSlash(a.Path), func(path string, d fs.DirEntry, walkErr error) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if walkErr != nil {
				return walkErr
			}
			visited++
			if visited > 2000 || outputBytes > 16384 {
				return fs.SkipAll
			}
			if d.IsDir() {
				if d.Name() == ".git" || d.Name() == ".cache" {
					return fs.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			f, e := root.Open(filepath.FromSlash(path))
			if e != nil {
				return e
			}
			defer f.Close()
			info, e := f.Stat()
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() || info.Size() > 65536 {
				return nil
			}
			scan := bufio.NewScanner(io.LimitReader(f, 65536))
			line := 0
			for scan.Scan() {
				if e := ctx.Err(); e != nil {
					return e
				}
				line++
				if strings.Contains(scan.Text(), a.Query) {
					matches = append(matches, SearchMatch{Path: path, Line: line, Text: scan.Text()})
					outputBytes += len(path) + len(scan.Text()) + 32
					if outputBytes > 16384 {
						return fs.SkipAll
					}
				}
			}
			return scan.Err()
		})
		if e != nil {
			return "", e
		}
		data, e := json.Marshal(matches)
		return string(data), e
	}
	return "", errors.New("unsupported file tool")
}
