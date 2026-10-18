package tools

import (
	"bots/internal/provider"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func call(name, args string) provider.Call {
	return provider.Call{ID: "call", Type: "function", Function: provider.Function{Name: name, Arguments: args}}
}
func TestGrantsAndFileBoundaries(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	os.Mkdir(root, 0700)
	os.Mkdir(outside, 0700)
	os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0600)
	os.WriteFile(filepath.Join(root, "ok.txt"), []byte("hello needle"), 0600)
	r, close, e := Files([]string{root})
	if e != nil {
		t.Fatal(e)
	}
	defer close()
	ctx := context.Background()
	grants := []string{"file-read", "file-search", "file-write"}
	for _, c := range []provider.Call{call("missing", `{}`), call("file-read", `{"root":0,"path":"ok.txt"}`)} {
		result := r.Call(ctx, nil, c)
		if result.Error == nil {
			t.Fatal("ungranted call accepted")
		}
	}
	for _, path := range []string{"../outside/secret.txt", filepath.Join(outside, "secret.txt"), "C:secret", `..\outside\secret.txt`} {
		args, _ := json.Marshal(FileArgs{Path: path})
		if result := r.Call(ctx, grants, call("file-read", string(args))); result.Error == nil {
			t.Fatal("escape accepted", path)
		}
	}
	if result := r.Call(ctx, grants, call("file-read", `{"root":0,"path":"ok.txt","unknown":true}`)); result.Error == nil || result.Error.Code != "invalid_arguments" {
		t.Fatal(result)
	}
	if result := r.Call(ctx, grants, call("file-read", `{"root":0,"path":"ok.txt"}`)); result.Output != "hello needle" {
		t.Fatal(result)
	}
	result := r.Call(ctx, grants, call("file-search", `{"root":0,"path":".","query":"needle"}`))
	var matches []SearchMatch
	if e = json.Unmarshal([]byte(result.Output), &matches); e != nil || len(matches) != 1 || matches[0].Path != "ok.txt" || matches[0].Line != 1 || matches[0].Text != "hello needle" {
		t.Fatal(result, e)
	}
	if result := r.Call(ctx, grants, call("file-write", `{"root":0,"path":"new.txt","content":"new"}`)); result.Error != nil {
		t.Fatal(result)
	}
	if result := r.Call(ctx, grants, call("file-write", `{"root":0,"path":"new.txt","content":"overwrite"}`)); result.Error == nil {
		t.Fatal("overwrite allowed")
	}
	link := filepath.Join(root, "link")
	if runtime.GOOS == "windows" {
		if out, e := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); e != nil {
			t.Fatalf("junction fixture: %v %s", e, out)
		}
	} else {
		if e = os.Symlink(outside, link); e != nil {
			t.Fatal(e)
		}
	}
	if result := r.Call(ctx, grants, call("file-read", `{"root":0,"path":"link/secret.txt"}`)); result.Error == nil {
		t.Fatal("link escape allowed")
	}
	if result := r.Call(ctx, grants, call("file-write", `{"root":0,"path":"link/created.txt","content":"escape"}`)); result.Error == nil {
		t.Fatal("link write escape allowed")
	}
	if _, e = os.Stat(filepath.Join(outside, "created.txt")); !os.IsNotExist(e) {
		t.Fatal("outside write occurred", e)
	}
}

type slow struct{}

func (slow) Definition() provider.ToolDefinition { return provider.ToolDefinition{Name: "slow"} }
func (slow) Validate(json.RawMessage) error      { return nil }
func (slow) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}
func TestTimeout(t *testing.T) {
	r := New()
	r.Timeout = 10 * time.Millisecond
	r.Register(slow{})
	result := r.Call(context.Background(), []string{"slow"}, call("slow", `{}`))
	if result.Error == nil || result.Error.Code != "timeout" {
		t.Fatal(result)
	}
}

func TestGoalWorkspaceVersionsPathOnlyAndConfinement(t *testing.T) {
	base := t.TempDir()
	workspace, other := filepath.Join(base, "goal"), filepath.Join(base, "source")
	os.Mkdir(workspace, 0700)
	os.Mkdir(other, 0700)
	r, close, e := WithWorkspace(workspace, []string{other}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer close()
	ctx, grants := context.Background(), []string{"file-read", "file-write"}
	write := func(args string) string {
		result := r.Call(ctx, grants, call("file-write", args))
		if result.Error != nil {
			t.Fatal(result.Error)
		}
		return result.Output
	}
	if got := write(`{"path":"out/index.html","content":"one"}`); got != "created out/index.html" {
		t.Fatal(got)
	}
	if got := write(`{"path":"out/index.html","content":"two"}`); got != "created out/index-v2.html" {
		t.Fatal(got)
	}
	if got := write(`{"path":"out/index.html","content":"three"}`); got != "created out/index-v3.html" {
		t.Fatal(got)
	}
	write(`{"path":"out/index.html","content":"replaced","overwrite":true}`)
	if data, _ := os.ReadFile(filepath.Join(workspace, "out", "index.html")); string(data) != "replaced" {
		t.Fatal("overwrite ignored")
	}
	if result := r.Call(ctx, grants, call("file-read", `{"path":"out/index-v2.html"}`)); result.Output != "two" {
		t.Fatal("path-only read failed", result)
	}
	if result := r.Call(ctx, grants, call("file-write", `{"root":1,"path":"x.txt","content":"a"}`)); result.Error != nil {
		t.Fatal(result.Error)
	}
	if result := r.Call(ctx, grants, call("file-write", `{"root":1,"path":"x.txt","content":"b"}`)); result.Error == nil {
		t.Fatal("non-workspace root must stay exclusive")
	}
	if result := r.Call(ctx, grants, call("file-write", `{"path":"../escape.txt","content":"x"}`)); result.Error == nil {
		t.Fatal("traversal allowed")
	}
}
