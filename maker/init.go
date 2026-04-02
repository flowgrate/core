package maker

import (
	"fmt"
	"os"
	"path/filepath"
)

const configTemplate = `database:
  # PostgreSQL connection string.
  # Can also be set via DATABASE_URL environment variable.
  url: %s

migrations:
  # Path to your SDK project (used by 'flowgrate make').
  project: ./Migrations

  # SDK language for template generation (used by 'flowgrate make').
  sdk: %s  # csharp | python

  # Command to invoke your SDK and output migration manifests to stdout.
  # Uncomment the line that matches your setup:
  #
  # run: dotnet run --project ./Migrations
  # run: python ./Migrations/runner.py
  # run: php artisan flowgrate:export
  # run: poetry run python ./Migrations/runner.py
  # run: bundle exec rake flowgrate:dump
  # run: docker compose exec sdk dotnet run --project /migrations
`

// InitConfig generates a flowgrate.yml in the given directory.
// Returns an error if the file already exists and force is false.
func InitConfig(dir, db, sdk string, force bool) (string, error) {
	path := filepath.Join(dir, "flowgrate.yml")

	if !force {
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("flowgrate.yml already exists (use --force to overwrite)")
		}
	}

	if db == "" {
		db = "postgres://user:pass@localhost/mydb"
	}
	if sdk == "" {
		sdk = "csharp"
	}

	content := fmt.Sprintf(configTemplate, db, sdk)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}
