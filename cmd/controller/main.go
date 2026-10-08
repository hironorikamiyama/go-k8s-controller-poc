package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hironorikamiyama/go-k8s-controller-poc/internal/config"
	"github.com/hironorikamiyama/go-k8s-controller-poc/internal/controller"
	k8s "github.com/hironorikamiyama/go-k8s-controller-poc/internal/kubernetes"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	client, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("failed to create Kubernetes client: %v", err)
	}

	log.Printf(
		"controller started: namespace=%s deployment=%s desired=%d interval=%s",
		cfg.Namespace,
		cfg.DeploymentName,
		cfg.DesiredReplicas,
		cfg.PollInterval,
	)

	// 起動直後に一度Reconcileする。
	reconcile := func() {
		err := controller.ReconcileDeployment(
			ctx,
			client,
			cfg.Namespace,
			cfg.DeploymentName,
			cfg.DesiredReplicas,
		)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("reconcile failed: %v", err)
			}
		}
	}

	reconcile()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("shutdown signal received; controller stopped")
			return

		case <-ticker.C:
			reconcile()
		}
	}
}
