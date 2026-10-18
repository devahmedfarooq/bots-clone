package provider

import (
	"bots/internal/config"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPProtocolAndStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			fmt.Fprint(w, `{"status":"ok"}`)
		case "/apply-template":
			fmt.Fprint(w, `{"prompt":"rendered"}`)
		case "/tokenize":
			fmt.Fprint(w, `{"tokens":[1,2,3]}`)
		case "/v1/chat/completions":
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":1,\"total_tokens\":4}}\n\ndata: [DONE]\n\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p := &Llama{URL: server.URL}
	ctx := context.Background()
	if e := p.Health(ctx); e != nil {
		t.Fatal(e)
	}
	s, _ := config.Resolve(config.Settings{}, config.Settings{})
	request := Request{Model: "base", Messages: []Message{{Role: "user", Content: "hello"}}, Settings: s}
	if n, e := p.Count(ctx, request); e != nil || n != 3 {
		t.Fatal(n, e)
	}
	var text strings.Builder
	result, e := p.Generate(ctx, request, func(s string) { text.WriteString(s) })
	if e != nil || text.String() != "hello" || result.Usage.Total != 4 || result.Finish != "stop" {
		t.Fatal(result, e, text.String())
	}
}
func TestCancelAndLaterGeneration(t *testing.T) {
	entered := make(chan struct{})
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		requests++
		if requests == 1 {
			close(entered)
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
			}
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"recovered"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()
	p := &Llama{URL: server.URL}
	settings, _ := config.Resolve(config.Settings{}, config.Settings{})
	request := Request{Model: "base", Settings: settings}
	result := make(chan error, 1)
	go func() { _, e := p.Generate(context.Background(), request, nil); result <- e }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("generation did not start")
	}
	p.Cancel()
	select {
	case e := <-result:
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not stop generation")
	}
	out, e := p.Generate(context.Background(), request, nil)
	if e != nil || out.Message.Content != "recovered" {
		t.Fatal(out, e)
	}
}
func TestInvalidLoadAndTruncatedStream(t *testing.T) {
	p := &Llama{ModelID: "base", Capacity: 4096}
	if p.Load(context.Background(), Model{ID: "wrong", ContextSize: 4096}) == nil {
		t.Fatal("wrong base accepted")
	}
	if p.Load(context.Background(), Model{ID: "base", ContextSize: 8192}) == nil {
		t.Fatal("oversized context accepted")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
	}))
	defer server.Close()
	p.URL = server.URL
	if _, e := p.Generate(context.Background(), Request{}, func(string) {}); e == nil {
		t.Fatal("truncated stream accepted")
	}
}

func TestPayloadRequestsSchemaOnlyWithoutTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)
	with := payload(Request{Model: "m", Schema: schema})
	format, ok := with["response_format"].(map[string]any)
	if !ok || format["type"] != "json_schema" {
		t.Fatalf("schema not sent: %v", with["response_format"])
	}
	if _, has := payload(Request{Model: "m"})["response_format"]; has {
		t.Fatal("no schema, no constraint")
	}
	if _, has := payload(Request{Model: "m", Schema: schema, Tools: []Tool{{}}})["response_format"]; has {
		t.Fatal("schema must not combine with tool calls")
	}
}
