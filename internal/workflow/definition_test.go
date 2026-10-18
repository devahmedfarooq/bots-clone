package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"bots/internal/config"
	"bots/internal/registry"
)

func TestWorkflowValidation(t *testing.T) {
	dir := t.TempDir()
	b := config.Bot{Schema: 1, ID: "one", Version: "1.0.0", Description: "test", Instructions: "test", BaseModel: "base", Budgets: config.Budgets{Tokens: 100, Seconds: 5}}
	data, _ := json.Marshal(b)
	os.WriteFile(filepath.Join(dir, "one.json"), data, 0600)
	reg := registry.Registry{Dir: dir}
	valid := Definition{Schema: 1, ID: "chain", Steps: []Step{{ID: "b", Bot: "one", DependsOn: []string{"a"}, InputFrom: "a", InputType: "text", OutputType: "text"}, {ID: "a", Bot: "one", Input: "hello", InputType: "text", OutputType: "text"}}}
	ordered, err := valid.Validate(reg)
	if err != nil || ordered[0].ID != "a" {
		t.Fatal(ordered, err)
	}
	for _, change := range []func(*Definition){
		func(d *Definition) { d.Steps[1].DependsOn = []string{"b"} },
		func(d *Definition) { d.Steps[0].InputType = "json" },
		func(d *Definition) { d.Steps[0].DependsOn = nil },
		func(d *Definition) { d.Steps[1].ID = "b" },
		func(d *Definition) { d.Steps[1].InputType = "json" },
	} {
		data, _ := json.Marshal(valid)
		var d Definition
		json.Unmarshal(data, &d)
		change(&d)
		if _, err := d.Validate(reg); err == nil {
			t.Fatal("invalid workflow accepted", d)
		}
	}
}
