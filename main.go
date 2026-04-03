package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/flowgrate/core/config"
	"github.com/flowgrate/core/maker"
	"github.com/flowgrate/core/manifest"
	"github.com/flowgrate/core/runner"
	"github.com/flowgrate/core/squash"
)

func main() {
	initCmd := flag.NewFlagSet("init", flag.ExitOnError)
	initDB := initCmd.String("db", "", "database URL to pre-fill")
	initSDK := initCmd.String("sdk", "csharp", "sdk to use (csharp | python)")
	initForce := initCmd.Bool("force", false, "overwrite existing flowgrate.yml")

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

	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
	statusDB := statusCmd.String("db", "", "PostgreSQL DSN (overrides config)")
	statusConfig := statusCmd.String("config", config.DefaultFile, "path to flowgrate.yml")

	freshCmd := flag.NewFlagSet("fresh", flag.ExitOnError)
	freshDB := freshCmd.String("db", "", "PostgreSQL DSN (overrides config)")
	freshConfig := freshCmd.String("config", config.DefaultFile, "path to flowgrate.yml")
	freshForce := freshCmd.Bool("force", false, "skip confirmation prompt (required when stdin is piped)")

	squashCmd := flag.NewFlagSet("squash", flag.ExitOnError)
	squashDB := squashCmd.String("db", "", "PostgreSQL DSN (overrides config)")
	squashConfig := squashCmd.String("config", config.DefaultFile, "path to flowgrate.yml")
	squashPrune := squashCmd.Bool("prune", false, "delete all migration files after dumping")

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "init":
		initCmd.Parse(os.Args[2:])
		path, err := maker.InitConfig(".", *initDB, *initSDK, *initForce)
		if err != nil {
			fatal("%v", err)
		}
		fmt.Printf("created: %s\n", path)

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
		path, err := maker.Make(name, cfg.Migrations.Project, cfg.Migrations.ResolvedSDK(),
			maker.MakeOptions{
				StubsDir:  cfg.Migrations.Stubs,
				FileExt:   cfg.Migrations.FileExt,
				TableCase: cfg.Migrations.TableCase,
			})
		if err != nil {
			fatal("%v", err)
		}
		fmt.Printf("created: %s\n", path)

	case "up":
		upCmd.Parse(os.Args[2:])
		runUp(*upConfig, *upDB, *upStep)

	case "down":
		downCmd.Parse(os.Args[2:])
		runDown(*downConfig, *downDB, *downStep)

	case "status":
		statusCmd.Parse(os.Args[2:])
		runStatus(*statusConfig, *statusDB)

	case "fresh":
		freshCmd.Parse(os.Args[2:])
		runFresh(*freshConfig, *freshDB, *freshForce)

	case "squash":
		squashCmd.Parse(os.Args[2:])
		runSquash(*squashConfig, *squashDB, *squashPrune)

	case "help", "--help", "-h":
		printHelp()

	default:
		fmt.Fprintf(os.Stderr, "error: unknown command: %s\n\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

func runUp(configFile, dbFlag string, step int) {
	cfg, dsn := loadConfig(configFile, dbFlag)
	migrations, err := collectMigrations(cfg)
	if err != nil {
		fatal("collect migrations: %v", err)
	}

	ctx := context.Background()
	r := connect(ctx, dsn)
	defer r.Close(ctx)

	// If a schema dump exists and the DB is fresh, load it instead of
	// running all migrations from scratch. This is the squash fast-path.
	state, stateErr := squash.New(dsn)
	if stateErr == nil {
		schemaFile := squash.SchemaFilePath(filepath.Dir(configFile), state.Driver())
		if _, err := os.Stat(schemaFile); err == nil {
			fresh, err := r.IsFresh(ctx)
			if err != nil {
				fatal("check db state: %v", err)
			}
			if fresh {
				fmt.Printf("loading schema dump: %s\n", schemaFile)
				if err := state.Load(schemaFile); err != nil {
					fatal("load schema dump: %v", err)
				}
			}
		}
	}

	if err := r.Init(ctx); err != nil {
		fatal("init: %v", err)
	}

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
			continue
		}
		if err := r.Up(ctx, m, batch); err != nil {
			fatal("%v", err)
		}
		fmt.Printf("applied: %s\n", m.Migration)
		applied++
	}
	if applied == 0 {
		fmt.Println("nothing to migrate")
	}
}

func runDown(configFile, dbFlag string, step int) {
	cfg, dsn := loadConfig(configFile, dbFlag)
	migrations, err := collectMigrations(cfg)
	if err != nil {
		fatal("collect migrations: %v", err)
	}

	ctx := context.Background()
	r := connect(ctx, dsn)
	defer r.Close(ctx)

	if err := r.Init(ctx); err != nil {
		fatal("init: %v", err)
	}

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
	if count == 0 {
		fmt.Println("nothing to roll back")
	}
}

func runStatus(configFile, dbFlag string) {
	cfg, dsn := loadConfig(configFile, dbFlag)
	migrations, err := collectMigrations(cfg)
	if err != nil {
		fatal("collect migrations: %v", err)
	}

	ctx := context.Background()
	r := connect(ctx, dsn)
	defer r.Close(ctx)

	if err := r.Init(ctx); err != nil {
		fatal("init: %v", err)
	}

	applied, err := r.ListApplied(ctx)
	if err != nil {
		fatal("list applied: %v", err)
	}

	// Build a lookup map: migration name → record
	appliedMap := make(map[string]runner.AppliedMigration, len(applied))
	for _, a := range applied {
		appliedMap[a.Migration] = a
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Migration\tStatus\tBatch\tApplied At")
	fmt.Fprintln(w, strings.Repeat("─", 80))

	for _, m := range migrations {
		if rec, ok := appliedMap[m.Migration]; ok {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
				m.Migration, "Applied", rec.Batch,
				rec.AppliedAt.Format(time.DateTime))
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				m.Migration, "Pending", "-", "-")
		}
	}
	w.Flush()
}

func runFresh(configFile, dbFlag string, force bool) {
	stdinIsPipe := stdinPiped()

	if !force && stdinIsPipe {
		fatal("stdin is piped; add --force to skip the confirmation prompt")
	}

	cfg, dsn := loadConfig(configFile, dbFlag)

	if !force {
		fmt.Printf("WARNING: This will drop ALL tables in the database and re-run all migrations.\n")
		fmt.Printf("Database: %s\n\n", dsn)
		fmt.Print(`Type "yes" to continue: `)
		var answer string
		fmt.Scanln(&answer)
		if answer != "yes" {
			fmt.Println("aborted")
			return
		}
	}

	migrations, err := collectMigrations(cfg)
	if err != nil {
		fatal("collect migrations: %v", err)
	}

	ctx := context.Background()
	r := connect(ctx, dsn)
	defer r.Close(ctx)

	fmt.Println("dropping all tables...")
	if err := r.DropAllTables(ctx); err != nil {
		fatal("drop tables: %v", err)
	}

	if err := r.Init(ctx); err != nil {
		fatal("init: %v", err)
	}

	batch := 1
	for _, m := range migrations {
		if err := r.Up(ctx, m, batch); err != nil {
			fatal("%v", err)
		}
		fmt.Printf("applied: %s\n", m.Migration)
	}
	fmt.Println("done")
}

func runSquash(configFile, dbFlag string, prune bool) {
	cfg, dsn := loadConfig(configFile, dbFlag)

	state, err := squash.New(dsn)
	if err != nil {
		fatal("%v", err)
	}

	schemaFile := squash.SchemaFilePath(filepath.Dir(configFile), state.Driver())

	fmt.Printf("dumping schema to %s...\n", schemaFile)
	if err := state.Dump(schemaFile); err != nil {
		fatal("dump: %v", err)
	}
	fmt.Println("schema dumped successfully")

	if prune {
		dir := cfg.Migrations.Project
		if dir == "" {
			fatal("migrations.project not set in config; cannot prune")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			fatal("read migrations dir: %v", err)
		}
		pruned := 0
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if err := os.Remove(path); err != nil {
				fatal("remove %s: %v", path, err)
			}
			pruned++
		}
		fmt.Printf("pruned %d migration file(s)\n", pruned)
	}
}

// --- Helpers ---

func loadConfig(configFile, dbFlag string) (*config.Config, string) {
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
	return cfg, dsn
}

func connect(ctx context.Context, dsn string) *runner.Runner {
	r, err := runner.New(ctx, dsn)
	if err != nil {
		fatal("connect: %v", err)
	}
	return r
}

func stdinPiped() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// collectMigrations reads manifests from stdin (if piped) or invokes the SDK.
func collectMigrations(cfg *config.Config) ([]manifest.Migration, error) {
	var src io.Reader

	if stdinPiped() {
		src = os.Stdin
	} else {
		shell, err := cfg.Migrations.RunCommand()
		if err != nil {
			return nil, err
		}
		cmd := exec.Command("sh", "-c", shell)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("run %q: %w\n%s", shell, err, stderr.String())
		}
		src = &stdout
	}

	var migrations []manifest.Migration
	decoder := json.NewDecoder(bufio.NewReader(src))
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
  init           Generate a flowgrate.yml config file in the current directory.
    --db=DSN       Pre-fill database URL
    --sdk=LANG     SDK language: csharp | python (default: csharp)
    --force        Overwrite existing flowgrate.yml

  make <Name>    Create a new migration file with a timestamp prefix.
                 The name determines the template:
                   CreateUsersTable       → Schema.Create
                   AddStatusToUsers       → Schema.Table + AddColumn
                   ChangeEmailInUsers     → Schema.Table + ChangeColumn
                   DropAvatarFromUsers    → Schema.Table + DropColumn
                   DropPostsTable         → Schema.DropIfExists
                 For custom/third-party SDKs a JSON skeleton (.migration) is
                 generated, or templates from migrations.stubs dir are used.

  up             Apply all pending migrations.
    --step=N       Apply only N migrations (default: all)
    --db=DSN       PostgreSQL DSN (overrides config)
    --config=FILE  Path to config file (default: flowgrate.yml)

  down           Roll back applied migrations.
    --step=N       Number of migrations to roll back (default: 1)
    --db=DSN       PostgreSQL DSN (overrides config)
    --config=FILE  Path to config file (default: flowgrate.yml)

  status         Show applied and pending migrations.
    --db=DSN       PostgreSQL DSN (overrides config)
    --config=FILE  Path to config file (default: flowgrate.yml)

  fresh          Drop all tables and re-run all migrations from scratch.
    --force        Skip confirmation prompt (required when stdin is piped)
    --db=DSN       PostgreSQL DSN (overrides config)
    --config=FILE  Path to config file (default: flowgrate.yml)

  squash         Dump current schema to schema/{driver}-schema.sql.
                 On next "flowgrate up" against an empty database, the dump
                 is loaded instead of replaying all migrations from scratch.
    --prune        Delete all migration files after dumping
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
  flowgrate status
  flowgrate fresh
  flowgrate fresh --force
`)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
