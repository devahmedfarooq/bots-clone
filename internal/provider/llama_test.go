package provider

import (
	"bots/internal/config"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
