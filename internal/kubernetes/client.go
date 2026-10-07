package kubernetes

import (
	"context"
	"fmt"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// NewClient creates a Kubernetes client using the local kubeconfig.
func NewClient() (*kubernetes.Clientset, error) {
	home := homedir.HomeDir()
	if home == "" {
		return nil, fmt.Errorf("home directory could not be determined")
	}

	kubeconfig := filepath.Join(home, ".kube", "config")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return clientset, nil
}

func ScaleDeployment(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	namespace string,
	name string,
	replicas int32,
) error {
	scale, err := clientset.
		AppsV1().
		Deployments(namespace).
		GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment scale: %w", err)
	}

	fmt.Printf(
		"Scaling deployment %s/%s: %d -> %d\n",
		namespace,
		name,
		scale.Spec.Replicas,
		replicas,
	)

	scale.Spec.Replicas = replicas

	_, err = clientset.
		AppsV1().
		Deployments(namespace).
		UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment scale: %w", err)
	}

	return nil
}

func ReconcileDeployment(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	namespace string,
	name string,
	desiredReplicas int32,
) error {
	scale, err := clientset.
		AppsV1().
		Deployments(namespace).
		GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment scale: %w", err)
	}

	currentReplicas := scale.Spec.Replicas

	fmt.Printf(
		"Reconciling deployment %s/%s: current=%d desired=%d\n",
		namespace,
		name,
		currentReplicas,
		desiredReplicas,
	)

	if currentReplicas == desiredReplicas {
		fmt.Println("No scaling required")
		return nil
	}

	return ScaleDeployment(
		ctx,
		clientset,
		namespace,
		name,
		desiredReplicas,
	)
}
