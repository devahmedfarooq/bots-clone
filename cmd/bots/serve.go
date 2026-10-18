package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"bots/internal/application"
	"bots/internal/lock"
	"bots/internal/provider"
	"bots/internal/runner"
	"bots/internal/service"
	"bots/internal/state"
)

func serve(ctx context.Context, c appConfig, catalog application.Catalog, args []string) error {
	f := flags("serve")
	address := f.String("listen", "127.0.0.1:8080", "listen address")
	limit := f.Int("queue", 8, "maximum waiting runs")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return errors.New("unexpected serve arguments")
	}
	ln, err := net.Listen("tcp", *address)
	if err != nil {
		return err
	}
	defer ln.Close()
	if !ln.Addr().(*net.TCPAddr).IP.IsLoopback() {
		return errors.New("service currently supports loopback addresses only")
	}
	if err = os.MkdirAll(c.Home, 0700); err != nil {
		return err
	}
	unlock, err := lock.Acquire(filepath.Join(c.Home, "inference.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	store, err := state.Open(filepath.Join(c.Home, "metadata.db"))
	if err != nil {
		return err
	}
	defer store.Close()
	p := &provider.Llama{Binary: c.Engine.Binary, ModelPath: c.Engine.ModelPath, ModelID: c.Engine.ModelID, BaseManifest: c.Engine.BaseManifest, RuntimeRevision: c.Engine.RuntimeRevision, CacheType: c.Engine.CacheType, Capacity: c.Engine.Context, GPU: c.Engine.GPU, Device: c.Engine.Device, LogPath: filepath.Join(c.Home, "engine.log")}
	installed, err := catalog.Registry.List()
	if err != nil {
		return err
	}
	for _, bot := range installed {
		if bot.Adapter != "" {
			p.AdapterPaths = append(p.AdapterPaths, bot.Adapter)
		}
	}
	manager := provider.NewManager(p)
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = manager.Close(shutdown)
	}()
	q, err := service.NewQueue(ctx, &runner.Runner{ContextLimit: c.Engine.Context, WorkspaceDir: filepath.Join(c.Home, "workspaces"), Models: manager, Store: store, Plugins: catalog.Plugins}, catalog, *limit)
	if err != nil {
		return err
	}
	defer q.Close()
	flows, err := service.NewWorkflows(q)
	if err != nil {
		return err
	}
	defer flows.Close()
	autonomy, err := service.NewAutonomy(q)
	if err != nil {
		return err
	}
	defer autonomy.Close()
	s := &service.Server{Catalog: catalog, Queue: q, Store: store, Workflows: flows, Autonomy: autonomy}
	server := &http.Server{Handler: service.Secure(s.Handler(), ln.Addr().String()), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
		case <-done:
		}
	}()
	fmt.Fprintln(os.Stderr, "Local dashboard: http://"+ln.Addr().String()+"/board")
	err = server.Serve(ln)
	close(done)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
