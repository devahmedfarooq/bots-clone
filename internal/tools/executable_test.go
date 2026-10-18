package tools

import (
	"context"
	"testing"

	"bots/internal/provider"
)

func TestNoArbitraryExecutionTools(t *testing.T) {
	r, close, err := Files(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	for _, name := range []string{"shell", "code", "exec", "powershell", "python"} {
		if _, err = r.Definitions([]string{name}); err == nil {
			t.Fatal("arbitrary executable exposed", name)
		}
		result := r.Call(context.Background(), []string{name}, provider.Call{Function: provider.Function{Name: name, Arguments: `{"command":"anything"}`}})
		if result.Error == nil || result.Error.Code != "unknown_tool" {
			t.Fatal(result)
		}
	}
}
