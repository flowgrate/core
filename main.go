package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/flowgrate/core/config"
	"github.com/flowgrate/core/maker"
	"github.com/flowgrate/core/manifest"
	"github.com/flowgrate/core/runner"
)

func main() {
	makeCmd := flag.NewFlagSet("make", flag.ExitOnError)
	makeConfig := makeCmd.String("config", config.DefaultFile, "path to flowgrate.yml")

	upCmd := flag.NewFlagSet("up", flag.ExitOnError)
	upDB := upCmd.String("db", "", "PostgreSQL DSN (overrides config)")
	upConfig := upCmd.String("config", config.DefaultFile, "path to flowgrate.yml")
	upStep := upCmd.Int("step", 0, "apply only N migrations (0 = all)")

	downCmd := flag.NewFlagSet("down", flag.ExitOnError)
	downDB := downCmd.String("db", "", "PostgreSQL DSN (overrides config)")
	downConfig := downCmd.String("config", config.DefaultFile, "path to flowgrate.yml")
	downStep := downCmd.Int("step", 1, "roll back N migrations (default 1)")

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	var direction string
	var dbFlag string
	var configFile string
	var step int

	switch os.Args[1] {
	case "make":
		makeCmd.Parse(os.Args[2:])
		if makeCmd.NArg() == 0 {
			fatal("usage: flowgrate make <MigrationName>")
		}
		cfg, err := config.Load(*makeConfig)
		if err != nil {
			fatal("%v", err)
		}
		name := makeCmd.Arg(0)
		path, err := maker.Make(name, cfg.Migrations.Project, cfg.Migrations.ResolvedSDK())
		if err != nil {
			fatal("%v", err)
		}
		fmt.Printf("created: %s\n", path)
		return
	case "up":
		upCmd.Parse(os.Args[2:])
		direction, dbFlag, configFile, step = "up", *upDB, *upConfig, *upStep
	case "down":
		downCmd.Parse(os.Args[2:])
		direction, dbFlag, configFile, step = "down", *downDB, *downConfig, *downStep
	case "help", "--help", "-h":
		printHelp()
		return
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command: %s\n\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		fatal("%v", err)
	}

	dsn := cfg.Database.URL
	if dbFlag != "" {
		dsn = dbFlag
	}
	if dsn == "" {
		fatal("no database DSN: set database.url in %s or use --db", configFile)
	}

	migrations, err := collectMigrations(cfg.Migrations.Project)
	if err != nil {
		fatal("collect migrations: %v", err)
	}

	ctx := context.Background()

	r, err := runner.New(ctx, dsn)
	if err != nil {
		fatal("connect: %v", err)
	}
	defer r.Close(ctx)

	if err := r.Init(ctx); err != nil {
		fatal("init: %v", err)
	}

	switch direction {
	case "up":
		batch, err := r.NextBatch(ctx)
		if err != nil {
			fatal("get batch: %v", err)
		}
		applied := 0
		for _, m := range migrations {
			if step > 0 && applied >= step {
				break
			}
			isApplied, err := r.IsApplied(ctx, m.Migration)
			if err != nil {
				fatal("%v", err)
			}
			if isApplied {
				fmt.Printf("skip: %s\n", m.Migration)
				continue
			}
			if err := r.Up(ctx, m, batch); err != nil {
				fatal("%v", err)
			}
			fmt.Printf("applied: %s\n", m.Migration)
			applied++
		}

	case "down":
		// Collect applied migrations in reverse order
		var toRollback []manifest.Migration
		for i := len(migrations) - 1; i >= 0; i-- {
			isApplied, err := r.IsApplied(ctx, migrations[i].Migration)
			if err != nil {
				fatal("%v", err)
			}
			if isApplied {
				toRollback = append(toRollback, migrations[i])
			}
		}
		count := 0
		for _, m := range toRollback {
			if count >= step {
				break
			}
			if err := r.Down(ctx, m); err != nil {
				fatal("%v", err)
			}
			fmt.Printf("rolled back: %s\n", m.Migration)
			count++
		}
	}
}

// collectMigrations reads manifests from stdin (if piped) or calls dotnet run.
func collectMigrations(project string) ([]manifest.Migration, error) {
	var src io.Reader

	stat, _ := os.Stdin.Stat()
	stdinIsPipe := (stat.Mode() & os.ModeCharDevice) == 0

	if stdinIsPipe {
		src = os.Stdin
	} else {
		cmd := exec.Command("dotnet", "run", "--project", project)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("dotnet run: %w\n%s", err, stderr.String())
		}
		src = &stdout
	}

	var migrations []manifest.Migration
	decoder := json.NewDecoder(src)
	for decoder.More() {
		var m manifest.Migration
		if err := decoder.Decode(&m); err != nil {
			return nil, fmt.Errorf("parse manifest: %w", err)
		}
		migrations = append(migrations, m)
	}

	return migrations, nil
}

func printHelp() {
	fmt.Print(`Flowgrate — Laravel-style migration library

Usage:
  flowgrate <command> [options]

Commands:
  make <Name>    Create a new migration file with a timestamp prefix.
                 The name determines the template:
                   CreateUsersTable       → Schema.Create
                   AddStatusToUsers       → Schema.Table + AddColumn
                   ChangeEmailInUsers     → Schema.Table + ChangeColumn
                   DropAvatarFromUsers    → Schema.Table + DropColumn
                   DropPostsTable         → Schema.DropIfExists

  up             Apply all pending migrations.
    --step=N       Apply only N migrations (default: all)
    --db=DSN       PostgreSQL DSN (overrides config)
    --config=FILE  Path to config file (default: flowgrate.yml)

  down           Roll back applied migrations.
    --step=N       Number of migrations to roll back (default: 1)
    --db=DSN       PostgreSQL DSN (overrides config)
    --config=FILE  Path to config file (default: flowgrate.yml)

  help           Show this help message.

Config (flowgrate.yml):
  database:
    url: postgres://user:pass@localhost/dbname

  migrations:
    project: ./Migrations   # path to SDK project
    sdk: csharp             # csharp | python

Examples:
  flowgrate make CreateUsersTable
  flowgrate up
  flowgrate up --step=3
  flowgrate down
  flowgrate down --step=3
`)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
