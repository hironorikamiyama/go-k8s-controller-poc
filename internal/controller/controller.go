package controller

import (
	"context"
	"fmt"

	k8s "github.com/hironorikamiyama/go-k8s-controller-poc/internal/kubernetes"
	"k8s.io/client-go/kubernetes"
)

func ReconcileDeployment(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, name string,
	desired int32,
) error {
	scale, err := k8s.GetScale(ctx, client, namespace, name)
	if err != nil {
		return err
	}

	current := scale.Spec.Replicas

	fmt.Printf(
		"Reconciling deployment %s/%s: current=%d desired=%d\n",
		namespace, name, current, desired,
	)

	if current == desired {
		fmt.Println("No scaling required")
		return nil
	}

	fmt.Printf(
		"Scaling deployment %s/%s: %d -> %d\n",
		namespace, name, current, desired,
	)

	return k8s.UpdateReplicas(
		ctx,
		client,
		namespace,
		name,
		scale,
		desired,
	)
}
