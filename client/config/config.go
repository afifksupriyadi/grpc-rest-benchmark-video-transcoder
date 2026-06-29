// Package config provides configuration management for the client.
package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config holds all configuration for the client.
type Config struct {
	GatewayRESTAddr string        `env:"GATEWAY_REST_ADDR" env-default:"localhost:8080"`
	GatewayGRPCAddr string        `env:"GATEWAY_GRPC_ADDR" env-default:"localhost:50051"`
	Timeout         time.Duration `env:"CLIENT_TIMEOUT" env-default:"5m"`
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
				panic(fmt.Sprintf("failed to load client config: %v", err))
			}
		}
	})
	return conf
}
