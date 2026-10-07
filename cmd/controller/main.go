package main

import (
	"context"
	"fmt"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8s "github.com/hironorikamiyama/go-k8s-controller-poc/internal/kubernetes"
)

func main() {
	clientset, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("failed to initialize kubernetes client: %v", err)
	}

	deployments, err := clientset.
		AppsV1().
		Deployments("").
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to list deployments: %v", err)
	}

	fmt.Printf("Found %d deployment(s)\n", len(deployments.Items))

	for _, deployment := range deployments.Items {
		fmt.Printf(
			"namespace=%s name=%s replicas=%d\n",
			deployment.Namespace,
			deployment.Name,
			deployment.Status.Replicas,
		)
	}

	fmt.Println("Scaling sample-app...")

	err = k8s.ScaleDeployment(
		context.Background(),
		clientset,
		"go-k8s-poc",
		"sample-app",
		3,
	)
	if err != nil {
		log.Fatalf("failed to scale deployment: %v", err)
	}

	fmt.Println("Scale request completed")

}
