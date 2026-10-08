package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetReplicas(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, name string,
) (int32, error) {
	scale, err := client.AppsV1().
		Deployments(namespace).
		GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("get scale %s/%s: %w", namespace, name, err)
	}

	return scale.Spec.Replicas, nil
}

func ScaleDeployment(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, name string,
	replicas int32,
) error {
	scale, err := client.AppsV1().
		Deployments(namespace).
		GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get scale %s/%s: %w", namespace, name, err)
	}

	scale.Spec.Replicas = replicas

	_, err = client.AppsV1().
		Deployments(namespace).
		UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update scale %s/%s: %w", namespace, name, err)
	}

	return nil
}
