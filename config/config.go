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
	Project    string `yaml:"project"`
	SDK        string `yaml:"sdk"`        // csharp | python | any custom value
	Run        string `yaml:"run"`        // shell command to invoke the SDK (required for custom SDKs)
	Stubs      string `yaml:"stubs"`      // path to directory with custom .tmpl files (optional)
	FileExt    string `yaml:"file_ext"`   // file extension for generated migration files (custom SDKs)
	TableCase  string `yaml:"table_case"` // snake (default) | camel
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
			"sdk %q requires migrations.run to be set in flowgrate.yml\n"+
				"  example:\n"+
				"    migrations:\n"+
				"      sdk: %s\n"+
				"      run: <command that prints JSON to stdout>\n"+
				"  spec: https://github.com/flowgrate/spec",
			m.ResolvedSDK(), m.ResolvedSDK())
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
