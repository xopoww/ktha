package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xopoww/ktha/node/internal/apps"
	"github.com/xopoww/ktha/node/internal/config"
	"go.uber.org/zap"
)

var args struct {
	config string

	// flags for temporary scenario
	image string
}

func init() {
	flag.StringVar(&args.config, "config", "/etc/ktha/node/config.yml", "path to yaml config file")

	// flags for temporary scenario
	flag.StringVar(&args.image, "image", "", "")

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

	// temporary hardcoded scenario

	if args.image == "" {
		return fmt.Errorf("image is required")
	}
	appCfg := apps.AppControllerConfig{
		Image:                 args.image,
		RunnerBinaryPath:      cfg.Runner.BinaryPath,
		NodeBinaryPath:        cfg.NodeJS.BinaryPath,
		RootfsRoot:            cfg.Runner.RootfsRoot,
		ReadinessPollInterval: time.Millisecond * 100,
		ReadinessTimeout:      time.Minute,
	}
	app := apps.NewAppController("app1", appCfg, log)

	if err := app.Start(); err != nil {
		return fmt.Errorf("start app: %w", err)
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigch
	log.Sugar().Infof("Received signal: %s.", sig)

	if err := app.Stop(time.Second * 5); err != nil {
		return fmt.Errorf("stop app: %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Fatal error: %s.", err)
	}
}
