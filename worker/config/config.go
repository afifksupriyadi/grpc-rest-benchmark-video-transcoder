// Package config provides configuration management for the worker service.
package config

import (
	"fmt"
	"sync"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config holds all configuration for the worker service.
type Config struct {
	Env         string `env:"ENV" env-default:"dev"`
	RESTPort    int    `env:"REST_PORT" env-default:"8081"`
	GRPCPort    int    `env:"GRPC_PORT" env-default:"50052"`
	MetricsPort int    `env:"METRICS_PORT" env-default:"2113"`
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
				panic(fmt.Sprintf("failed to load worker config: %v", err))
			}
		}
	})
	return conf
}
