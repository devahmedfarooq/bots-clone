package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Llama struct {
	Binary, ModelPath, ModelID, LogPath, Device string
	Capacity                                    int
	GPU                                         int
	URL                                         string
	Client                                      *http.Client
	mu                                          sync.Mutex
	cmd                                         *exec.Cmd
	done                                        chan error
	log                                         *os.File
	cancel                                      context.CancelFunc
	loaded                                      bool
	loadedContext                               int
	Starts                                      int
}

func (p *Llama) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return http.DefaultClient
}
func (p *Llama) Load(ctx context.Context, model Model) error {
	if model.ID != p.ModelID {
		return fmt.Errorf("model %q does not match configured base %q", model.ID, p.ModelID)
	}
	if model.ContextSize < 512 || model.ContextSize > p.Capacity {
		return errors.New("requested context exceeds configured engine capacity")
	}
	if p.loaded {
		if e := p.Health(ctx); e == nil && p.loadedContext == model.ContextSize {
			return nil
		}
		_ = p.Unload(context.Background())
	}
	if p.Binary == "" || p.ModelPath == "" {
		return errors.New("configure llama-server binary and local GGUF model")
	}
	if _, err := os.Stat(p.ModelPath); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	p.URL = "http://127.0.0.1:" + strconv.Itoa(port)
	if err = os.MkdirAll(filepath.Dir(p.LogPath), 0700); err != nil {
		return err
	}
	p.log, err = os.OpenFile(p.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	capacity := model.ContextSize
	p.cmd = exec.Command(p.Binary, "-m", p.ModelPath, "--alias", p.ModelID, "--host", "127.0.0.1", "--port", strconv.Itoa(port), "-c", strconv.Itoa(capacity), "-np", "1", "-ngl", strconv.Itoa(p.GPU), "--jinja", "--offline", "--no-webui", "--cache-ram", "0", "--log-verbosity", "2")
	if p.Device != "" {
		p.cmd.Args = append(p.cmd.Args, "--device", p.Device, "--split-mode", "none")
	}
	p.cmd.Stdout = p.log
	p.cmd.Stderr = p.log
	if err = p.cmd.Start(); err != nil {
		p.log.Close()
		p.log = nil
		return fmt.Errorf("start llama-server: %w", err)
	}
	p.done = make(chan error, 1)
	p.Starts++
	cmd := p.cmd
	go func() { p.done <- cmd.Wait() }()
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = p.Unload(context.Background())
			return ctx.Err()
		case <-timer.C:
			_ = p.Unload(context.Background())
			return errors.New("model loading timed out; inspect runtime log")
		case err = <-p.done:
			p.cmd = nil
			p.log.Close()
			p.log = nil
			return fmt.Errorf("llama-server exited: %v; inspect %s", err, p.LogPath)
		case <-ticker.C:
			probe, cancel := context.WithTimeout(ctx, time.Second)
			err = p.Health(probe)
			cancel()
			if err == nil {
				p.loaded = true
				p.loadedContext = model.ContextSize
				return nil
			}
		}
	}
}
func (p *Llama) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.URL+"/health", nil)
	if err != nil {
		return err
	}
	res, err := p.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("health: HTTP %d", res.StatusCode)
	}
	return nil
}
func (p *Llama) post(ctx context.Context, path string, data any) (*http.Response, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", p.URL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := p.client().Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		res.Body.Close()
		return nil, fmt.Errorf("inference %s: HTTP %d", path, res.StatusCode)
	}
	return res, nil
}
func payload(r Request) map[string]any {
	b := map[string]any{"model": r.Model, "messages": r.Messages, "max_tokens": r.Settings.MaxTokens, "temperature": r.Settings.Temperature, "seed": 42, "cache_prompt": false, "chat_template_kwargs": map[string]any{"enable_thinking": false}}
	if len(r.Tools) > 0 {
		b["tools"] = r.Tools
		b["parallel_tool_calls"] = false
	}
	return b
}
func (p *Llama) Count(ctx context.Context, r Request) (int, error) {
	if p.Capacity > 0 && r.Settings.ContextSize > p.Capacity {
		return 0, errors.New("requested context exceeds engine capacity; update framework.json and restart")
	}
	res, err := p.post(ctx, "/apply-template", payload(r))
	if err != nil {
		return 0, err
	}
	var rendered struct {
		Prompt string `json:"prompt"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&rendered)
	res.Body.Close()
	if err != nil {
		return 0, err
	}
	res, err = p.post(ctx, "/tokenize", map[string]any{"content": rendered.Prompt, "add_special": false, "parse_special": true})
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	var tokens struct {
		Tokens []int `json:"tokens"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&tokens); err != nil {
		return 0, err
	}
	return len(tokens.Tokens), nil
}
func (p *Llama) Generate(ctx context.Context, r Request, emit func(string)) (Response, error) {
	ctx, cancel := context.WithCancel(ctx)
	p.mu.Lock()
	p.cancel = cancel
	p.mu.Unlock()
	defer func() { cancel(); p.mu.Lock(); p.cancel = nil; p.mu.Unlock() }()
	body := payload(r)
	if emit != nil {
		body["stream"] = true
		body["stream_options"] = map[string]bool{"include_usage": true}
	}
	res, err := p.post(ctx, "/v1/chat/completions", body)
	if err != nil {
		return Response{}, err
	}
	defer res.Body.Close()
	if emit == nil {
		var v struct {
			Choices []struct {
				Message Message `json:"message"`
				Finish  string  `json:"finish_reason"`
			} `json:"choices"`
			Usage Usage `json:"usage"`
		}
		if err = json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&v); err != nil {
			return Response{}, err
		}
		if len(v.Choices) != 1 {
			return Response{}, errors.New("invalid model response: expected one choice")
		}
		return Response{v.Choices[0].Message, v.Usage, v.Choices[0].Finish}, nil
	}
	out := Response{Message: Message{Role: "assistant"}}
	calls := map[int]*Call{}
	scanner := bufio.NewScanner(io.LimitReader(res.Body, 8<<20))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	complete := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			complete = true
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
					Calls   []struct {
						Index    int `json:"index"`
						ID, Type string
						Function Function `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				Finish string `json:"finish_reason"`
			} `json:"choices"`
			Usage Usage `json:"usage"`
		}
		if err = json.Unmarshal([]byte(data), &chunk); err != nil {
			return out, err
		}
		if chunk.Usage.Total > 0 {
			out.Usage = chunk.Usage
		}
		for _, c := range chunk.Choices {
			out.Message.Content += c.Delta.Content
			emit(c.Delta.Content)
			if c.Finish != "" {
				out.Finish = c.Finish
			}
			for _, part := range c.Delta.Calls {
				call := calls[part.Index]
				if call == nil {
					call = &Call{}
					calls[part.Index] = call
				}
				if part.ID != "" {
					call.ID = part.ID
				}
				if part.Type != "" {
					call.Type = part.Type
				}
				call.Function.Name += part.Function.Name
				call.Function.Arguments += part.Function.Arguments
			}
		}
	}
	if err = scanner.Err(); err != nil {
		return out, err
	}
	if !complete {
		return out, errors.New("incomplete inference stream")
	}
	for i := 0; i < len(calls); i++ {
		if calls[i] == nil {
			return out, errors.New("invalid tool call index")
		}
		out.Message.ToolCalls = append(out.Message.ToolCalls, *calls[i])
	}
	return out, nil
}
func (p *Llama) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
	}
}
func (p *Llama) Unload(ctx context.Context) error {
	p.Cancel()
	p.loaded = false
	if p.cmd == nil {
		return nil
	}
	err := p.cmd.Process.Kill()
	select {
	case <-p.done:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return errors.New("model shutdown timed out")
	}
	p.cmd = nil
	if p.log != nil {
		p.log.Close()
		p.log = nil
	}
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
