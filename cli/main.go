package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/asciimoth/gonnect-vpn-example/cfg"
	"github.com/asciimoth/gonnect-vpn-example/logger"
	"github.com/asciimoth/gonnect-vpn-example/runner"
)

func main() {
	var logger logger.Logger = log.New(os.Stdout, "", log.Ltime|log.Lmicroseconds)
	cfg := cfg.Load()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	session, err := runner.Start(ctx, cfg, logger)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Stop()
	session.Wait()
}
