package config

import (
	"strings"

	"github.com/spf13/viper"
)

const (
	StaticDir = "static"
)

type environment string

const (
	EnvLocal environment = "localhost"
	EnvDev   environment = "development"
	EnvProd  environment = "production"
)

type (
	Config struct {
		HTTP     HTTPConfig
		App      AppConfig
		Database DatabaseConfig
	}

	HTTPConfig struct {
		Host string
		Port uint16
	}

	AppConfig struct {
		Env environment
	}

	DatabaseConfig struct{}
)

func LoadConfig() (Config, error) {
	var config Config

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
    viper.AddConfigPath("config")
    viper.AddConfigPath("../config")
    viper.AddConfigPath("../../config")

	viper.SetEnvPrefix("app")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		return config, err
	}

	if err := viper.Unmarshal(&config); err != nil {
		return config, err
	}
	return config, nil
}
