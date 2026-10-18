package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSwitchProtocolAndRecovery(t *testing.T) {
	state := []adapterScale{{0, 0}, {1, 0}}
	fail := false
	erased := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slots/0":
			erased++
			fmt.Fprint(w, `{"n_erased":42}`)
		case "/lora-adapters":
			if r.Method == "POST" {
				var next []adapterScale
				json.NewDecoder(r.Body).Decode(&next)
				if fail && next[1].Scale == 1 {
					http.Error(w, "out of memory", 500)
					return
				}
				state = next
			}
			json.NewEncoder(w).Encode(state)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p := &Llama{URL: server.URL, adapters: []loadedAdapter{{path: "a"}, {path: "b"}}}
	for _, path := range []string{"a", "b", ""} {
		if e := p.selectAdapter(context.Background(), path); e != nil {
			t.Fatal(e)
		}
		if p.active != path {
			t.Fatal("wrong active")
		}
	}
	fail = true
	if e := p.selectAdapter(context.Background(), "b"); e == nil {
		t.Fatal("failed activation accepted")
	}
	if p.active != "" || state[0].Scale != 0 || state[1].Scale != 0 || erased < 5 {
		t.Fatal("failed rollback", p.active, state, erased)
	}
}

type adapterFake struct {
	Provider
	active  string
	entered chan struct{}
	fail    bool
}

func (f *adapterFake) Load(context.Context, Model) error             { return nil }
func (f *adapterFake) ValidateAdapter(context.Context, string) error { return nil }
func (f *adapterFake) LoadAdapter(context.Context, string) error     { return nil }
func (f *adapterFake) ActivateAdapter(_ context.Context, s string) error {
	if f.fail {
		f.active = ""
		return errors.New("oom")
	}
	f.active = s
	return nil
}
func (f *adapterFake) DeactivateAdapter(context.Context) error     { f.active = ""; return nil }
func (f *adapterFake) UnloadAdapter(context.Context, string) error { return nil }
func TestSwitchWaitsForRunAndBaseReset(t *testing.T) {
	f := &adapterFake{}
	m := NewManager(f)
	release, e := m.Acquire(context.Background(), Model{Adapter: "a"})
	if e != nil {
		t.Fatal(e)
	}
	acquired := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(acquired)
		r, e := m.Acquire(context.Background(), Model{Adapter: "b"})
		if e == nil {
			if f.active != "b" {
				e = errors.New("wrong adapter")
			}
			r()
		}
		done <- e
	}()
	<-acquired
	select {
	case <-done:
		t.Fatal("switched during active run")
	case <-time.After(20 * time.Millisecond):
	}
	if f.active != "a" {
		t.Fatal("weights changed")
	}
	release()
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	r, e := m.Acquire(context.Background(), Model{})
	if e != nil {
		t.Fatal(e)
	}
	if f.active != "" {
		t.Fatal("base inherited adapter")
	}
	r()
	f.fail = true
	if _, e = m.Acquire(context.Background(), Model{Adapter: "bad"}); e == nil {
		t.Fatal("bad adapter accepted")
	}
	f.fail = false
	r, e = m.Acquire(context.Background(), Model{})
	if e != nil {
		t.Fatal(e)
	}
	r()
}
