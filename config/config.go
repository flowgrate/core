package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultFile = "flowgrate.yml"

type Config struct {
	Database   DatabaseConfig   `yaml:"database"`
	Migrations MigrationsConfig `yaml:"migrations"`
}

type DatabaseConfig struct {
	URL string `yaml:"url"`
}

type MigrationsConfig struct {
	Project string `yaml:"project"`
	SDK     string `yaml:"sdk"` // csharp | python (default: csharp)
}

func (m MigrationsConfig) ResolvedSDK() string {
	if m.SDK == "" {
		return "csharp"
	}
	return m.SDK
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if cfg.Database.URL == "" {
		if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
			cfg.Database.URL = dsn
		}
	}

	return &cfg, nil
}
