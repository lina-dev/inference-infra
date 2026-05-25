package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inference-infra/scheduler/internal/scheduler"
	"go.uber.org/zap"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	s, err := scheduler.New(ctx, log)
	if err != nil {
		log.Fatal("failed to create scheduler", zap.Error(err))
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	log.Info("scheduler started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Reconcile(ctx); err != nil {
				log.Error("reconcile error", zap.Error(err))
			}
		}
	}
}
