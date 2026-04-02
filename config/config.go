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
	Run     string `yaml:"run"` // optional: full command to invoke the SDK
}

func (m MigrationsConfig) ResolvedSDK() string {
	if m.SDK == "" {
		return "csharp"
	}
	return m.SDK
}

// RunCommand returns the shell command used to invoke the SDK.
// Priority: explicit run → SDK default → error.
// The command is always executed via "sh -c" so any shell syntax is supported.
func (m MigrationsConfig) RunCommand() (string, error) {
	if m.Run != "" {
		return m.Run, nil
	}

	switch m.ResolvedSDK() {
	case "csharp":
		return "dotnet run --project " + m.Project, nil
	case "python":
		return "python " + m.Project, nil
	default:
		return "", fmt.Errorf(
			"unknown sdk %q and no 'run' command set in flowgrate.yml\n"+
				"  add: migrations.run: <command>", m.ResolvedSDK())
	}
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
