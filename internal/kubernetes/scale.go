package kubernetes

import (
	"context"
	"fmt"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetScale retrieves the current Scale object.
func GetScale(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, name string,
) (*autoscalingv1.Scale, error) {
	scale, err := client.AppsV1().
		Deployments(namespace).
		GetScale(ctx, name, metav1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf(
			"get scale %s/%s: %w",
			namespace, name, err,
		)
	}

	return scale, nil
}

// UpdateReplicas updates an already retrieved Scale object.
func UpdateReplicas(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, name string,
	scale *autoscalingv1.Scale,
	replicas int32,
) error {
	if scale == nil {
		return fmt.Errorf("scale object is nil")
	}

	updated := scale.DeepCopy()
	updated.Spec.Replicas = replicas

	_, err := client.AppsV1().
		Deployments(namespace).
		UpdateScale(ctx, name, updated, metav1.UpdateOptions{})

	if err != nil {
		return fmt.Errorf(
			"update scale %s/%s: %w",
			namespace, name, err,
		)
	}

	return nil
}
