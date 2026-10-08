package controller

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestReconcileDeployment(t *testing.T) {
	const (
		namespace       = "go-k8s-poc"
		name            = "sample-app"
		resourceVersion = "12345"
	)

	tests := []struct {
		name          string
		current       int32
		desired       int32
		notFound      bool
		updateFailure bool
		conflict      bool
		wantErr       bool
		wantConflict  bool
		wantGets      int
		wantUpdates   int
	}{
		{
			name:    "already reconciled",
			current: 3, desired: 3,
			wantGets: 1, wantUpdates: 0,
		},
		{
			name:    "scale up",
			current: 1, desired: 3,
			wantGets: 1, wantUpdates: 1,
		},
		{
			name:    "scale down",
			current: 5, desired: 3,
			wantGets: 1, wantUpdates: 1,
		},
		{
			name:    "deployment not found",
			desired: 3, notFound: true,
			wantErr:  true,
			wantGets: 1, wantUpdates: 0,
		},
		{
			name:    "scale update failure",
			current: 1, desired: 3,
			updateFailure: true,
			wantErr:       true,
			wantGets:      1, wantUpdates: 1,
		},
		{
			name:    "resource version conflict",
			current: 1, desired: 3,
			conflict: true,
			wantErr:  true, wantConflict: true,
			wantGets: 1, wantUpdates: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []runtime.Object

			if !tt.notFound {
				current := tt.current
				objects = append(objects, &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:            name,
						Namespace:       namespace,
						ResourceVersion: resourceVersion,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &current,
					},
				})
			}

			client := fake.NewClientset(objects...)

			// Reactorが返したScaleを保持し、
			// UpdateReplicasが元オブジェクトを変更しないことを検証する。
			var originalScale *autoscalingv1.Scale

			client.PrependReactor(
				"get", "deployments",
				func(action k8stesting.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() != "scale" {
						return false, nil, nil
					}

					if tt.notFound {
						return true, nil, apierrors.NewNotFound(
							schema.GroupResource{
								Group:    "apps",
								Resource: "deployments",
							},
							name,
						)
					}

					originalScale = &autoscalingv1.Scale{
						ObjectMeta: metav1.ObjectMeta{
							Name:            name,
							Namespace:       namespace,
							ResourceVersion: resourceVersion,
						},
						Spec: autoscalingv1.ScaleSpec{
							Replicas: tt.current,
						},
					}

					return true, originalScale, nil
				},
			)

			client.PrependReactor(
				"update", "deployments",
				func(action k8stesting.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() != "scale" {
						return false, nil, nil
					}

					update, ok := action.(k8stesting.UpdateAction)
					if !ok {
						t.Fatalf("unexpected update action type: %T", action)
					}

					scale, ok := update.GetObject().(*autoscalingv1.Scale)
					if !ok {
						t.Fatalf(
							"unexpected update object type: %T",
							update.GetObject(),
						)
					}

					if scale.Spec.Replicas != tt.desired {
						t.Errorf(
							"updated replicas = %d, want %d",
							scale.Spec.Replicas,
							tt.desired,
						)
					}

					if scale.ResourceVersion != resourceVersion {
						t.Errorf(
							"resourceVersion = %q, want %q",
							scale.ResourceVersion,
							resourceVersion,
						)
					}

					if tt.conflict {
						return true, nil, apierrors.NewConflict(
							schema.GroupResource{
								Group:    "apps",
								Resource: "deployments",
							},
							name,
							errors.New("simulated resourceVersion conflict"),
						)
					}

					if tt.updateFailure {
						return true, nil, errors.New("simulated update failure")
					}

					return true, scale.DeepCopy(), nil
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

			if got := apierrors.IsConflict(err); got != tt.wantConflict {
				t.Errorf(
					"IsConflict(error) = %v, want %v",
					got,
					tt.wantConflict,
				)
			}

			gets := 0
			updates := 0

			for _, action := range client.Actions() {
				if action.GetSubresource() != "scale" {
					continue
				}

				if action.Matches("get", "deployments") {
					gets++
				}

				if action.Matches("update", "deployments") {
					updates++
				}
			}

			if gets != tt.wantGets {
				t.Errorf(
					"GetScale calls = %d, want %d",
					gets,
					tt.wantGets,
				)
			}

			if updates != tt.wantUpdates {
				t.Errorf(
					"UpdateScale calls = %d, want %d",
					updates,
					tt.wantUpdates,
				)
			}

			if originalScale != nil &&
				originalScale.Spec.Replicas != tt.current {
				t.Errorf(
					"original Scale replicas changed: got %d, want %d",
					originalScale.Spec.Replicas,
					tt.current,
				)
			}
		})
	}
}
