package informer

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"
)

const (
	testNamespace  = "go-k8s-poc"
	testDeployment = "sample-app"
	testKey        = testNamespace + "/" + testDeployment
)

func testDeploymentObject(
	namespace, name string,
	replicas int32,
) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
}

func TestDeploymentEventHandler(t *testing.T) {
	tests := []struct {
		name       string
		event      string
		oldObj     interface{}
		newObj     interface{}
		wantQueued bool
	}{
		{
			name:       "Add target deployment",
			event:      "add",
			newObj:     testDeploymentObject(testNamespace, testDeployment, 3),
			wantQueued: true,
		},
		{
			name:       "Update replicas 3 to 1",
			event:      "update",
			oldObj:     testDeploymentObject(testNamespace, testDeployment, 3),
			newObj:     testDeploymentObject(testNamespace, testDeployment, 1),
			wantQueued: true,
		},
		{
			name:       "Update without replica change",
			event:      "update",
			oldObj:     testDeploymentObject(testNamespace, testDeployment, 3),
			newObj:     testDeploymentObject(testNamespace, testDeployment, 3),
			wantQueued: false,
		},
		{
			name:       "Ignore other deployment",
			event:      "add",
			newObj:     testDeploymentObject(testNamespace, "other-app", 3),
			wantQueued: false,
		},
		{
			name:       "Ignore other namespace",
			event:      "add",
			newObj:     testDeploymentObject("other-ns", testDeployment, 3),
			wantQueued: false,
		},
		{
			name:       "Ignore invalid add object",
			event:      "add",
			newObj:     "invalid",
			wantQueued: false,
		},
		{
			name:       "Ignore invalid update object",
			event:      "update",
			oldObj:     "invalid",
			newObj:     testDeploymentObject(testNamespace, testDeployment, 1),
			wantQueued: false,
		},
		{
			name:       "Ignore delete event",
			event:      "delete",
			newObj:     testDeploymentObject(testNamespace, testDeployment, 3),
			wantQueued: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := workqueue.NewTyped[string]()
			defer queue.ShutDown()

			handler := newDeploymentEventHandler(
				testNamespace,
				testDeployment,
				queue,
			)

			switch tt.event {
			case "add":
				handler.OnAdd(tt.newObj, false)
			case "update":
				handler.OnUpdate(tt.oldObj, tt.newObj)
			case "delete":
				handler.OnDelete(tt.newObj)
			default:
				t.Fatalf("unknown event: %s", tt.event)
			}

			if got := queue.Len() > 0; got != tt.wantQueued {
				t.Fatalf(
					"queued=%v, want=%v",
					got,
					tt.wantQueued,
				)
			}

			if tt.wantQueued {
				key, shutdown := queue.Get()
				if shutdown {
					t.Fatal("queue unexpectedly shut down")
				}

				if key != testKey {
					t.Fatalf("key=%q, want=%q", key, testKey)
				}

				queue.Done(key)
			}
		})
	}
}

func TestWatchDeploymentInitialEventAndShutdown(t *testing.T) {
	client := fake.NewSimpleClientset(
		testDeploymentObject(testNamespace, testDeployment, 3),
	)

	queue := workqueue.NewTyped[string]()
	defer queue.ShutDown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- watchDeployment(
			ctx,
			client,
			testNamespace,
			testDeployment,
			queue,
			ready,
		)
	}()

	// Wait until the initial informer cache sync is complete.
	select {
	case <-ready:
		// Cache synchronization succeeded.
	case err := <-done:
		t.Fatalf("informer stopped before cache sync: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for informer cache sync")
	}

	// The initial Add event must have enqueued the target Deployment.
	if got := queue.Len(); got != 1 {
		t.Fatalf("queue length=%d, want=1", got)
	}

	key, shutdown := queue.Get()
	if shutdown {
		t.Fatal("queue unexpectedly shut down")
	}

	if key != testKey {
		queue.Done(key)
		t.Fatalf("key=%q, want=%q", key, testKey)
	}

	queue.Done(key)

	// Shutdown must complete after cancellation.
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watchDeployment returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("informer did not stop after context cancellation")
	}
}
