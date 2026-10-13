package worker

import (
	"context"
	"log"

	"github.com/hironorikamiyama/go-k8s-controller-poc/internal/controller"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const MaxRetries = 5

func Run(
	ctx context.Context,
	client kubernetes.Interface,
	queue workqueue.TypedRateLimitingInterface[string],
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
				queue.Forget(key)
				return
			}

			if itemNamespace != namespace || itemName != deploymentName {
				log.Printf("[Worker] ignored unexpected key: %s", key)
				queue.Forget(key)
				return
			}

			log.Printf(
				"[Worker] reconciling %s attempt=%d",
				key,
				queue.NumRequeues(key)+1,
			)

			err = controller.ReconcileDeployment(
				ctx,
				client,
				itemNamespace,
				itemName,
				desiredReplicas,
			)

			if err == nil {
				queue.Forget(key)
				return
			}

			if ctx.Err() != nil {
				queue.Forget(key)
				return
			}

			if queue.NumRequeues(key) >= MaxRetries {
				log.Printf(
					"[Worker] max retries exceeded: key=%s err=%v",
					key,
					err,
				)
				queue.Forget(key)
				return
			}

			queue.AddRateLimited(key)

			log.Printf(
				"[Worker] reconcile failed; retry scheduled: key=%s retry=%d err=%v",
				key,
				queue.NumRequeues(key),
				err,
			)
		}()
	}
}
