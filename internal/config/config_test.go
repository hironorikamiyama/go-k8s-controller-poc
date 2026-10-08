package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		want       Config
		wantErr    bool
		errMessage string
	}{
		{
			name: "default configuration",
			want: Config{
				Namespace:       "go-k8s-poc",
				DeploymentName:  "sample-app",
				DesiredReplicas: 3,
				PollInterval:    5 * time.Second,
			},
		},
		{
			name: "custom configuration",
			env: map[string]string{
				"NAMESPACE":        "test-ns",
				"DEPLOYMENT_NAME":  "test-app",
				"DESIRED_REPLICAS": "5",
				"POLL_INTERVAL":    "2s",
			},
			want: Config{
				Namespace:       "test-ns",
				DeploymentName:  "test-app",
				DesiredReplicas: 5,
				PollInterval:    2 * time.Second,
			},
		},
		{
			name: "zero replicas allowed",
			env: map[string]string{
				"DESIRED_REPLICAS": "0",
			},
			want: Config{
				Namespace:       "go-k8s-poc",
				DeploymentName:  "sample-app",
				DesiredReplicas: 0,
				PollInterval:    5 * time.Second,
			},
		},
		{
			name: "negative replicas",
			env: map[string]string{
				"DESIRED_REPLICAS": "-1",
			},
			wantErr:    true,
			errMessage: "DESIRED_REPLICAS",
		},
		{
			name: "replicas overflow",
			env: map[string]string{
				"DESIRED_REPLICAS": "2147483648",
			},
			wantErr:    true,
			errMessage: "DESIRED_REPLICAS",
		},
		{
			name: "invalid replicas",
			env: map[string]string{
				"DESIRED_REPLICAS": "abc",
			},
			wantErr:    true,
			errMessage: "DESIRED_REPLICAS",
		},
		{
			name: "zero polling interval",
			env: map[string]string{
				"POLL_INTERVAL": "0s",
			},
			wantErr:    true,
			errMessage: "POLL_INTERVAL",
		},
		{
			name: "negative polling interval",
			env: map[string]string{
				"POLL_INTERVAL": "-5s",
			},
			wantErr:    true,
			errMessage: "POLL_INTERVAL",
		},
		{
			name: "invalid polling interval",
			env: map[string]string{
				"POLL_INTERVAL": "invalid",
			},
			wantErr:    true,
			errMessage: "POLL_INTERVAL",
		},
		{
			name: "empty namespace",
			env: map[string]string{
				"NAMESPACE": "",
			},
			wantErr:    true,
			errMessage: "NAMESPACE",
		},
		{
			name: "empty deployment name",
			env: map[string]string{
				"DEPLOYMENT_NAME": "",
			},
			wantErr:    true,
			errMessage: "DEPLOYMENT_NAME",
		},
	}

	keys := []string{
		"NAMESPACE",
		"DEPLOYMENT_NAME",
		"DESIRED_REPLICAS",
		"POLL_INTERVAL",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 外部環境変数の影響を受けないよう初期化する。
			for _, key := range keys {
				t.Setenv(key, "")
			}

			// デフォルト値を使用する項目は環境変数を未設定にする。
			// t.Setenvではunsetできないため、
			// デフォルト値を明示してテストする。
			defaults := map[string]string{
				"NAMESPACE":        "go-k8s-poc",
				"DEPLOYMENT_NAME":  "sample-app",
				"DESIRED_REPLICAS": "3",
				"POLL_INTERVAL":    "5s",
			}

			for key, value := range defaults {
				t.Setenv(key, value)
			}

			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			got, err := Load()

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"Load() error = %v, wantErr = %v",
					err,
					tt.wantErr,
				)
			}

			if tt.wantErr {
				if err != nil && !strings.Contains(err.Error(), tt.errMessage) {
					t.Errorf(
						"error = %q, want substring %q",
						err.Error(),
						tt.errMessage,
					)
				}
				return
			}

			if got != tt.want {
				t.Errorf(
					"Load() = %+v, want %+v",
					got,
					tt.want,
				)
			}
		})
	}
}
