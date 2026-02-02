package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	MySQL     MySQLConfig     `mapstructure:"mysql"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Etcd      EtcdConfig      `mapstructure:"etcd"`
	Workers   WorkersConfig   `mapstructure:"workers"`
	Stream    StreamConfig    `mapstructure:"stream"`
	Auth      AuthConfig      `mapstructure:"auth"`
	RateLimit RateLimitConfig `mapstructure:"ratelimit"`
}

type ServerConfig struct {
	Environment string `mapstructure:"environment"`
	Port        string `mapstructure:"port"`
}

type MySQLConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type EtcdConfig struct {
	Endpoints   []string      `mapstructure:"endpoints"`
	DialTimeout time.Duration `mapstructure:"dial_timeout"`
}

type WorkersConfig struct {
	OutboxInterval       time.Duration `mapstructure:"outbox_interval"`
	ReconcilerInterval   time.Duration `mapstructure:"reconciler_interval"`
	ReconcilerBatchSize  int           `mapstructure:"reconciler_batch_size"`
	ReconcilerBatchDelay time.Duration `mapstructure:"reconciler_batch_delay"`
}

type StreamConfig struct {
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	HubBufferSize     int           `mapstructure:"hub_buffer_size"`
}

type AuthConfig struct {
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
}

type RateLimitConfig struct {
	RequestsPerSecond int `mapstructure:"requests_per_second"`
}

func Load() *Config {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.SetEnvPrefix("MIZU")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		// Just log if config file is not found, relying on Env or simply empty values if that's intended (though risky without defaults)
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		panic(err)
	}

	return &cfg
}
