// Package config provides configuration management for the gateway service.
package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// WorkerConfig holds addresses for both REST and gRPC worker instances.
type WorkerConfig struct {
	Worker1RESTAddr string `env:"WORKER_1_REST_ADDR" env-default:"localhost:8081"`
	Worker2RESTAddr string `env:"WORKER_2_REST_ADDR" env-default:"localhost:8082"`
	Worker1GRPCAddr string `env:"WORKER_1_GRPC_ADDR" env-default:"localhost:50052"`
	Worker2GRPCAddr string `env:"WORKER_2_GRPC_ADDR" env-default:"localhost:50053"`
}

// HTTPClientConfig holds HTTP client settings for outbound REST calls to Worker.
type HTTPClientConfig struct {
	Timeout time.Duration `env:"HTTP_CLIENT_TIMEOUT" env-default:"60s"`
}

// Config holds all configuration for the gateway service.
type Config struct {
	Env         string `env:"ENV" env-default:"dev"`
	RESTPort    int    `env:"REST_PORT" env-default:"8080"`
	GRPCPort    int    `env:"GRPC_PORT" env-default:"50051"`
	MetricsPort int    `env:"METRICS_PORT" env-default:"2112"`

	Worker     WorkerConfig
	HTTPClient HTTPClientConfig
}

var (
	conf *Config
	once sync.Once
)

// Get returns the singleton Config instance loaded from .env or environment variables.
func Get() *Config {
	once.Do(func() {
		conf = &Config{}
		if err := cleanenv.ReadConfig(".env", conf); err != nil {
			if err := cleanenv.ReadEnv(conf); err != nil {
				panic(fmt.Sprintf("failed to load gateway config: %v", err))
			}
		}
	})
	return conf
}
