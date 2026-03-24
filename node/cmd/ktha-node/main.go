package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xopoww/ktha/node/internal/apps"
	"github.com/xopoww/ktha/node/internal/config"
	"github.com/xopoww/ktha/node/internal/proxy"
	"go.uber.org/zap"
)

var args struct {
	config string

	// flags for temporary scenario
	image1 string
	image2 string
}

func init() {
	flag.StringVar(&args.config, "config", "/etc/ktha/node/config.yml", "path to yaml config file")

	// flags for temporary scenario
	flag.StringVar(&args.image1, "image1", "", "")
	flag.StringVar(&args.image2, "image2", "", "")

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

	// set up manager

	mgrCfg := apps.AppManagerConfig{
		RunnerBinaryPath:      cfg.Runner.BinaryPath,
		NodeBinaryPath:        cfg.NodeJS.BinaryPath,
		RootfsRoot:            cfg.Runner.RootfsRoot,
		ReadinessPollInterval: time.Millisecond * 100,
		ReadinessTimeout:      time.Minute,
		IdleTimeout:           time.Second * 10,
	}
	mgr := apps.NewAppManager(mgrCfg, log)

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

	// temporary hardcoded scenario

	if args.image1 == "" || args.image2 == "" {
		return fmt.Errorf("images are required")
	}
	if err := mgr.AddApp("app1", apps.AppSpec{
		Image: args.image1,
	}); err != nil {
		log.Sugar().Errorf("Failed to add app1: %s.", err)
	}
	if err := mgr.AddApp("app2", apps.AppSpec{
		Image: args.image2,
	}); err != nil {
		log.Sugar().Errorf("Failed to add app2: %s.", err)
	}

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
	mgr.Shutdown(time.Second * 5)

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Fatal error: %s.", err)
	}
}
