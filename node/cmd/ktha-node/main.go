package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/xopoww/ktha/node/internal/admin"
	"github.com/xopoww/ktha/node/internal/config"
	"github.com/xopoww/ktha/node/internal/manager"
	"github.com/xopoww/ktha/node/internal/metrics"
	"github.com/xopoww/ktha/node/internal/proxy"
	"go.uber.org/zap"
)

var args struct {
	config string
}

func init() {
	flag.StringVar(&args.config, "config", "/etc/ktha/node/config.yml", "path to yaml config file")

	flag.Parse()
}

func run() error {
	log, err := zap.NewDevelopment()
	if err != nil {
		return fmt.Errorf("init zap: %w", err)
	}
	defer log.Sync()
	l := log.Sugar()

	cfg, err := config.Load(args.config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// set up root cgroup subtree

	if err := os.MkdirAll(cfg.Runner.CgroupBasePath, 0o755); err != nil {
		return fmt.Errorf("mkdir root cgroup: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(cfg.Runner.CgroupBasePath, "cgroup.subtree_control"),
		[]byte("+memory +pids +cpu"),
		0o644,
	); err != nil {
		return fmt.Errorf("enable cgroup controllers: %w", err)
	}

	// set up dummy nodejs process to fault in shared pages

	dummyNodejs := exec.Command(cfg.Runner.NodeJS.BinaryPath, "-e", "setInterval(()=>{},2**31-1)")
	l.Debugf("Running dummy nodejs: %s.", dummyNodejs)
	if err := dummyNodejs.Start(); err != nil {
		return fmt.Errorf("start dummy nodejs: %w", err)
	}
	defer func() {
		if err := dummyNodejs.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
			l.Warnf("Failed to signal dummy nodejs: %s.", err)
		}
	}()
	go func() {
		if err := dummyNodejs.Wait(); err != nil {
			finalErr := err

			if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
					finalErr = nil
				}
			}

			if finalErr != nil {
				l.Warnf("Dummy nodejs errored: %s.", finalErr)
			} else {
				l.Debugf("Dummy nodejs exited (%s).", err)
			}
		}
	}()

	// set up manager

	mgrCfg := manager.AppManagerConfig{
		ImagesBasePath: cfg.Application.ImagesBasePath,
		Runner:         cfg.Runner,
		Limits:         cfg.Application.Limits,
		Readiness:      cfg.Application.Readiness,
		Timeouts:       cfg.Application.Timeouts,
	}
	mgr := manager.NewAppManager(mgrCfg, l)

	specs := make([]manager.AppSpec, 0, len(cfg.Application.Apps))
	for id, appCfg := range cfg.Application.Apps {
		specs = append(specs, manager.AppSpec{
			ID:    id,
			Image: appCfg.Image,
			Env:   appCfg.Env,
		})
	}
	metrics.Registry().MustRegister(mgr)

	if err := mgr.AddApps(specs); err != nil {
		return fmt.Errorf("add apps: %w", err)
	}

	// set up admin server

	adminServer := admin.NewAdminServer(admin.AdminDeps{
		Cfg: cfg.Admin,
		Mgr: mgr,
		L:   l,
	})
	go func() {
		l.Debugf("Starting the admin server on %s...", adminServer.Addr)
		err := adminServer.ListenAndServe()
		l.Infof("Admin server stopped: %s.", err)
	}()

	// set up proxy

	proxyServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Proxy.Port),
		Handler: proxy.NewReverseProxy(mgr, l),
	}
	go func() {
		l.Debugf("Starting the reverse proxy on %s...", proxyServer.Addr)
		err := proxyServer.ListenAndServe()
		l.Infof("Reverse proxy stopped: %s.", err)
	}()

	// graceful shutdown

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigch
	l.Infof("Received signal: %s.", sig)

	// first, shutdown and drain the reverse proxy
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := proxyServer.Shutdown(ctx); err != nil {
		l.Errorf("Error during reverse proxy shutdown: %s.", err)
	}

	// then, shutdown the guests
	mgr.Shutdown()

	// shutdown the admin server
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := adminServer.Shutdown(ctx); err != nil {
		l.Errorf("Error during admin server shutdown: %s.", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Fatal error: %s.", err)
	}
}
