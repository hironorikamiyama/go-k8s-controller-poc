package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"
)

const (
	testNamespace  = "go-k8s-poc"
	testDeployment = "sample-app"
	testKey        = testNamespace + "/" + testDeployment
)

func newTestQueue() workqueue.TypedRateLimitingInterface[string] {
	return workqueue.NewTypedRateLimitingQueue(
		workqueue.NewTypedItemExponentialFailureRateLimiter[string](
			1*time.Millisecond,
			5*time.Millisecond,
		),
	)
}

func newTestClient() *fake.Clientset {
	client := fake.NewSimpleClientset()

	// ReconcileDeployment が呼び出す GetScale を再現する。
	client.PrependReactor(
		"get",
		"deployments",
		func(action ktesting.Action) (bool, runtime.Object, error) {
			return true, &autoscalingv1.Scale{
				ObjectMeta: metav1.ObjectMeta{
					Name:            testDeployment,
					Namespace:       testNamespace,
					ResourceVersion: "1",
				},
				Spec: autoscalingv1.ScaleSpec{
					Replicas: 3,
				},
			}, nil
		},
	)

	return client
}

func runTestWorker(
	t *testing.T,
	ctx context.Context,
	client *fake.Clientset,
	queue workqueue.TypedRateLimitingInterface[string],
) <-chan struct{} {
	t.Helper()

	done := make(chan struct{})

	go func() {
		defer close(done)

		Run(
			ctx,
			client,
			queue,
			testNamespace,
			testDeployment,
			3,
		)
	}()

	return done
}

func waitForCount(
	t *testing.T,
	count *atomic.Int32,
	expected int32,
) {
	t.Helper()

	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		if count.Load() >= expected {
			return
		}

		select {
		case <-deadline:
			t.Fatalf(
				"timeout: expected at least %d calls, got %d",
				expected,
				count.Load(),
			)
		case <-ticker.C:
		}
	}
}

func stopWorker(
	t *testing.T,
	queue workqueue.TypedRateLimitingInterface[string],
	done <-chan struct{},
) {
	t.Helper()

	queue.ShutDown()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestWorkerSuccess(t *testing.T) {
	client := newTestClient()
	queue := newTestQueue()
	ctx := context.Background()

	var calls atomic.Int32

	client.PrependReactor(
		"get",
		"deployments",
		func(action ktesting.Action) (bool, runtime.Object, error) {
			calls.Add(1)
			return false, nil, nil
		},
	)

	done := runTestWorker(t, ctx, client, queue)

	queue.Add(testKey)
	waitForCount(t, &calls, 1)

	// queue.Done / Forget の完了を待つため、停止させる。
	stopWorker(t, queue, done)

	if got := queue.NumRequeues(testKey); got != 0 {
		t.Fatalf("expected 0 requeues, got %d", got)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 GetScale call, got %d", got)
	}
}

func TestWorkerRetriesAfterAPIError(t *testing.T) {
	client := newTestClient()
	queue := newTestQueue()
	ctx := context.Background()

	var calls atomic.Int32

	client.PrependReactor(
		"get",
		"deployments",
		func(action ktesting.Action) (bool, runtime.Object, error) {
			if calls.Add(1) == 1 {
				return true, nil, errors.New("temporary API failure")
			}
			return false, nil, nil
		},
	)

	done := runTestWorker(t, ctx, client, queue)

	queue.Add(testKey)
	waitForCount(t, &calls, 2)

	stopWorker(t, queue, done)

	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 GetScale calls, got %d", got)
	}
}

func TestWorkerResetsRetriesAfterRecovery(t *testing.T) {
	client := newTestClient()
	queue := newTestQueue()
	ctx := context.Background()

	var calls atomic.Int32

	client.PrependReactor(
		"get",
		"deployments",
		func(action ktesting.Action) (bool, runtime.Object, error) {
			if calls.Add(1) <= 2 {
				return true, nil, errors.New("temporary API failure")
			}
			return false, nil, nil
		},
	)

	done := runTestWorker(t, ctx, client, queue)

	queue.Add(testKey)
	waitForCount(t, &calls, 3)

	stopWorker(t, queue, done)

	if got := queue.NumRequeues(testKey); got != 0 {
		t.Fatalf("expected retry history reset, got %d", got)
	}
}

func TestWorkerStopsAfterMaxRetries(t *testing.T) {
	client := newTestClient()
	queue := newTestQueue()
	ctx := context.Background()

	var calls atomic.Int32

	client.PrependReactor(
		"get",
		"deployments",
		func(action ktesting.Action) (bool, runtime.Object, error) {
			calls.Add(1)
			return true, nil, errors.New("persistent API failure")
		},
	)

	done := runTestWorker(t, ctx, client, queue)

	queue.Add(testKey)
	waitForCount(t, &calls, int32(MaxRetries+1))

	// 最終失敗の処理が完了するまで待機する。
	deadline := time.After(3 * time.Second)
	for queue.NumRequeues(testKey) != 0 {
		select {
		case <-deadline:
			t.Fatal("retry history was not cleared")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	stopWorker(t, queue, done)

	if got := calls.Load(); got != int32(MaxRetries+1) {
		t.Fatalf(
			"expected %d calls, got %d",
			MaxRetries+1,
			got,
		)
	}
}

func TestWorkerShutdown(t *testing.T) {
	client := newTestClient()
	queue := newTestQueue()
	ctx := context.Background()

	done := runTestWorker(t, ctx, client, queue)

	stopWorker(t, queue, done)
}
