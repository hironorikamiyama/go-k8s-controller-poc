package controller

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestReconcileDeployment(t *testing.T) {
	const (
		namespace = "go-k8s-poc"
		name      = "sample-app"
	)

	tests := []struct {
		name          string
		current       int32
		desired       int32
		notFound      bool
		updateFailure bool
		wantErr       bool
		wantUpdates   int
	}{
		{
			name:    "already reconciled",
			current: 3, desired: 3,
			wantUpdates: 0,
		},
		{
			name:    "scale up",
			current: 1, desired: 3,
			wantUpdates: 1,
		},
		{
			name:    "scale down",
			current: 5, desired: 3,
			wantUpdates: 1,
		},
		{
			name:    "deployment not found",
			desired: 3, notFound: true,
			wantErr: true,
		},
		{
			name:    "scale update failure",
			current: 1, desired: 3,
			updateFailure: true,
			wantErr:       true, wantUpdates: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []runtime.Object

			if !tt.notFound {
				objects = append(objects, &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &tt.current,
					},
				})
			}

			client := fake.NewClientset(objects...)

			// fake clientのScale subresourceを模擬する。
			// client-goのバージョン差異に依存しないよう
			// 明示的にGetScale/UpdateScaleを処理する。
			client.PrependReactor(
				"get", "deployments",
				func(action k8stesting.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() != "scale" {
						return false, nil, nil
					}
					if tt.notFound {
						return true, nil, errors.New("deployment not found")
					}
					return true, &autoscalingv1.Scale{
						ObjectMeta: metav1.ObjectMeta{
							Name: name, Namespace: namespace,
						},
						Spec: autoscalingv1.ScaleSpec{
							Replicas: tt.current,
						},
					}, nil
				},
			)

			client.PrependReactor(
				"update", "deployments",
				func(action k8stesting.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() != "scale" {
						return false, nil, nil
					}
					if tt.updateFailure {
						return true, nil, errors.New("simulated update failure")
					}
					update := action.(k8stesting.UpdateAction)
					return true, update.GetObject(), nil
				},
			)

			err := ReconcileDeployment(
				context.Background(),
				client,
				namespace,
				name,
				tt.desired,
			)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}

			updates := 0
			for _, action := range client.Actions() {
				if action.Matches("update", "deployments") &&
					action.GetSubresource() == "scale" {
					updates++
				}
			}

			if updates != tt.wantUpdates {
				t.Errorf(
					"scale updates = %d, want %d",
					updates,
					tt.wantUpdates,
				)
			}
		})
	}
}
