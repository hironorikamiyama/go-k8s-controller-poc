package informer

import (
	"context"
	"fmt"
	"log"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func WatchDeployment(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	deploymentName string,
	queue workqueue.TypedInterface[string],
) error {
	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		0,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(
			func(options *metav1.ListOptions) {
				options.FieldSelector = "metadata.name=" + deploymentName
			},
		),
	)

	deploymentInformer := factory.Apps().
		V1().
		Deployments().
		Informer()

	_, err := deploymentInformer.AddEventHandler(
		newDeploymentEventHandler(namespace, deploymentName, queue),
	)
	if err != nil {
		return fmt.Errorf("add deployment event handler: %w", err)
	}

	factory.Start(ctx.Done())

	if !cache.WaitForCacheSync(
		ctx.Done(),
		deploymentInformer.HasSynced,
	) {
		return fmt.Errorf("deployment informer cache sync failed")
	}

	log.Printf("[Informer] watching %s/%s", namespace, deploymentName)

	<-ctx.Done()
	return nil
}

func newDeploymentEventHandler(
	namespace string,
	deploymentName string,
	queue workqueue.TypedInterface[string],
) cache.ResourceEventHandler {
	enqueue := func(obj interface{}) {
		deployment, ok := obj.(*appsv1.Deployment)
		if !ok {
			return
		}

		// FieldSelectorに加えて、イベント側でも対象を検証する。
		if deployment.Namespace != namespace ||
			deployment.Name != deploymentName {
			return
		}

		key, err := cache.MetaNamespaceKeyFunc(deployment)
		if err != nil {
			log.Printf("[Informer] key generation failed: %v", err)
			return
		}

		queue.Add(key)
		log.Printf("[Informer] enqueued %s", key)
	}

	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			enqueue(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeployment, oldOK := oldObj.(*appsv1.Deployment)
			newDeployment, newOK := newObj.(*appsv1.Deployment)
			if !oldOK || !newOK {
				return
			}

			if replicas(oldDeployment) == replicas(newDeployment) {
				return
			}

			enqueue(newDeployment)
		},
	}
}

func replicas(deployment *appsv1.Deployment) int32 {
	if deployment.Spec.Replicas == nil {
		return 1
	}
	return *deployment.Spec.Replicas
}
