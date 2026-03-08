package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	RepoPath       string `mapstructure:"repo_path"`
	Theme          string `mapstructure:"theme"`
	RefreshSeconds int    `mapstructure:"refresh_seconds"`
	DefaultWindow  string `mapstructure:"default_window"`
}

func Default() Config {
	return Config{
		RepoPath:       ".",
		Theme:          "tokyo-night",
		RefreshSeconds: 60,
		DefaultWindow:  "30d",
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetDefault("repo_path", cfg.RepoPath)
	v.SetDefault("theme", cfg.Theme)
	v.SetDefault("refresh_seconds", cfg.RefreshSeconds)
	v.SetDefault("default_window", cfg.DefaultWindow)

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	return cfg, nil
}
