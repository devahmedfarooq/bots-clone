package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryRegistration(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "package")
	os.Mkdir(pkg, 0700)
	data := `{"schema_version":1,"id":"demo","version":"1.0.0","description":"test","instructions":"help","base_model":"base","tools":[],"context_sources":["."],"generation":{},"budgets":{"tokens":1000,"tool_calls":0,"seconds":30},"evaluation_suite":""}`
	os.WriteFile(filepath.Join(pkg, "bot.json"), []byte(data), 0600)
	r := Registry{filepath.Join(root, "registry")}
	b, e := r.Register(pkg)
	expected, canonicalErr := filepath.EvalSymlinks(pkg)
	if e != nil || canonicalErr != nil || b.ContextSources[0] != expected {
		t.Fatal(b, e)
	}
	if _, e = r.Register(pkg); e == nil {
		t.Fatal("duplicate accepted")
	}
	list, e := r.List()
	if e != nil || len(list) != 1 {
		t.Fatal(list, e)
	}
	if _, e = r.Get("../demo"); e == nil {
		t.Fatal("escape ID accepted")
	}
	if e = r.Remove("demo"); e != nil {
		t.Fatal(e)
	}
	list, e = r.List()
	if e != nil || len(list) != 0 {
		t.Fatal(list, e)
	}
}
