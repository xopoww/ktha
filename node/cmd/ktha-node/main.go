package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/xopoww/ktha/node/internal/apps"
	"github.com/xopoww/ktha/node/internal/config"
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

	cfg, err := config.Load(args.config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// set up root cgroup subtree

	if err := os.MkdirAll(cfg.Runner.CgroupRoot, 0o755); err != nil {
		return fmt.Errorf("mkdir root cgroup: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(cfg.Runner.CgroupRoot, "cgroup.subtree_control"),
		[]byte("+memory +pids +cpu"),
		0o644,
	); err != nil {
		return fmt.Errorf("enable cgroup controllers: %w", err)
	}

	// set up manager

	mgrCfg := apps.AppManagerConfig{
		RunnerBinaryPath:      cfg.Runner.BinaryPath,
		NodeBinaryPath:        cfg.NodeJS.BinaryPath,
		RootfsRoot:            cfg.Runner.RootfsRoot,
		CgroupRoot:            cfg.Runner.CgroupRoot,
		ImagesBasePath:        cfg.Application.ImagesBasePath,
		Limits:                cfg.Application.Limits,
		ReadinessPollInterval: cfg.Application.ReadinessPollInterval,
		ReadinessTimeout:      cfg.Application.ReadinessTimeout,
		IdleTimeout:           cfg.Application.IdleTimeout,
		StopTimeout:           cfg.Application.StopTimeout,
	}
	mgr := apps.NewAppManager(mgrCfg, log)

	specs := make([]apps.AppSpec, 0, len(cfg.Application.Apps))
	for id, appCfg := range cfg.Application.Apps {
		specs = append(specs, apps.AppSpec{
			ID:    id,
			Image: appCfg.Image,
		})
	}
	if err := mgr.AddApps(specs); err != nil {
		return fmt.Errorf("add apps: %w", err)
	}

	// set up proxy

	proxyServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Proxy.Port),
		Handler: proxy.NewReverseProxy(mgr, log),
	}
	go func() {
		log.Sugar().Debugf("Starting the reverse proxy on %s...", proxyServer.Addr)
		err := proxyServer.ListenAndServe()
		log.Sugar().Infof("Reverse proxy stopped: %s.", err)
	}()

	// graceful shutdown

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigch
	log.Sugar().Infof("Received signal: %s.", sig)

	// first, shutdown and drain the reverse proxy
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := proxyServer.Shutdown(ctx); err != nil {
		log.Sugar().Errorf("Error during reverse proxy shutdown: %s.", err)
	}

	// then, shutdown the guests
	mgr.Shutdown()

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Fatal error: %s.", err)
	}
}
