package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hironorikamiyama/go-k8s-controller-poc/internal/config"
	"github.com/hironorikamiyama/go-k8s-controller-poc/internal/informer"
	k8s "github.com/hironorikamiyama/go-k8s-controller-poc/internal/kubernetes"
	"github.com/hironorikamiyama/go-k8s-controller-poc/internal/worker"
	"k8s.io/client-go/util/workqueue"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
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

	queue := workqueue.NewTyped[string]()

	log.Printf(
		"controller started: namespace=%s deployment=%s desired=%d mode=event-driven",
		cfg.Namespace,
		cfg.DeploymentName,
		cfg.DesiredReplicas,
	)

	// WorkerはInformerより先に起動してよい。
	// queue.Get() はイベントが届くまで待機する。
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)

		worker.Run(
			ctx,
			client,
			queue,
			cfg.Namespace,
			cfg.DeploymentName,
			cfg.DesiredReplicas,
		)
	}()

	// Informerは同期完了まで待機し、以降はイベントを監視する。
	informerDone := make(chan error, 1)
	go func() {
		informerDone <- informer.WatchDeployment(
			ctx,
			client,
			cfg.Namespace,
			cfg.DeploymentName,
			queue,
		)
	}()

	select {
	case <-ctx.Done():
		log.Println("shutdown signal received")
	case err := <-informerDone:
		if err != nil {
			log.Printf("informer stopped with error: %v", err)
		} else {
			log.Println("informer stopped")
		}
	}

	stop()
	queue.ShutDown()
	<-workerDone

	log.Println("controller stopped")
}
