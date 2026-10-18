package plugin

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestProtocol(t *testing.T) {
	for _, input := range []string{`{"version":2,"arguments":{}}`, `{"version":1,"arguments":{}} {}`, strings.Repeat("x", Limit+1)} {
		if err := Serve(strings.NewReader(input), &bytes.Buffer{}, func(json.RawMessage) (string, error) { t.Fatal("invalid request executed"); return "", nil }); err == nil {
			t.Fatal("invalid protocol accepted")
		}
	}
	var output bytes.Buffer
	if err := Serve(strings.NewReader(`{"version":1,"arguments":{"text":"hello"}}`), &output, func(raw json.RawMessage) (string, error) { return "hello", nil }); err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := json.Unmarshal(output.Bytes(), &response); err != nil || response.Version != Version || response.Output != "hello" {
		t.Fatal(output.String(), err)
	}
}
