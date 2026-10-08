package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Namespace       string
	DeploymentName  string
	DesiredReplicas int32
	PollInterval    time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Namespace:      getEnv("NAMESPACE", "go-k8s-poc"),
		DeploymentName: getEnv("DEPLOYMENT_NAME", "sample-app"),
		PollInterval:   5 * time.Second,
	}

	replicas, err := strconv.ParseInt(
		getEnv("DESIRED_REPLICAS", "3"),
		10,
		32,
	)
	if err != nil || replicas < 0 {
		return Config{}, fmt.Errorf(
			"DESIRED_REPLICAS must be an integer between 0 and 2147483647",
		)
	}
	cfg.DesiredReplicas = int32(replicas)

	interval, err := time.ParseDuration(
		getEnv("POLL_INTERVAL", "5s"),
	)
	if err != nil || interval <= 0 {
		return Config{}, fmt.Errorf(
			"POLL_INTERVAL must be a positive duration, e.g. 5s",
		)
	}
	cfg.PollInterval = interval

	if cfg.Namespace == "" || cfg.DeploymentName == "" {
		return Config{}, fmt.Errorf(
			"NAMESPACE and DEPLOYMENT_NAME must not be empty",
		)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
