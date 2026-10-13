package worker

import (
	"context"
	"log"

	"github.com/hironorikamiyama/go-k8s-controller-poc/internal/controller"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func Run(
	ctx context.Context,
	client kubernetes.Interface,
	queue workqueue.TypedInterface[string],
	namespace string,
	deploymentName string,
	desiredReplicas int32,
) {
	for {
		key, shutdown := queue.Get()
		if shutdown {
			log.Println("[Worker] queue shutdown")
			return
		}

		func() {
			defer queue.Done(key)

			itemNamespace, itemName, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				log.Printf("[Worker] invalid key %q: %v", key, err)
				return
			}

			if itemNamespace != namespace || itemName != deploymentName {
				log.Printf("[Worker] ignored unexpected key: %s", key)
				return
			}

			log.Printf("[Worker] reconciling %s", key)

			err = controller.ReconcileDeployment(
				ctx,
				client,
				itemNamespace,
				itemName,
				desiredReplicas,
			)
			if err != nil && ctx.Err() == nil {
				log.Printf("[Worker] reconcile failed: %v", err)
			}
		}()
	}
}
