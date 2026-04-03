# flowgrate

Free, open-source Laravel-style migration CLI. Works with any language SDK that outputs the [Flowgrate JSON manifest](https://github.com/flowgrate/spec).

## How it works

```
Your SDK (C#, Python, ...) → JSON manifest → flowgrate CLI → SQL → Database
```

The CLI reads migration manifests from your SDK project (via stdout pipe or direct invocation), compiles them to SQL, and executes them in a transaction.

## Installation

### Download binary

Grab the latest release from [Releases](https://github.com/flowgrate/core/releases) for your platform:

```bash
# Linux (amd64)
curl -L https://github.com/flowgrate/core/releases/latest/download/flowgrate-linux-amd64 -o flowgrate
chmod +x flowgrate
sudo mv flowgrate /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/flowgrate/core/releases/latest/download/flowgrate-darwin-arm64 -o flowgrate
chmod +x flowgrate
sudo mv flowgrate /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/flowgrate/core/releases/latest/download/flowgrate-darwin-amd64 -o flowgrate
chmod +x flowgrate
sudo mv flowgrate /usr/local/bin/
```

### Build from source

```bash
git clone https://github.com/flowgrate/core
cd core
go build -o flowgrate .
```

## Quick start

**1. Generate config:**

```bash
flowgrate init --db=postgres://user:pass@localhost/mydb --sdk=csharp
```

Or create `flowgrate.yml` manually:

```yaml
database:
  url: postgres://user:pass@localhost/mydb

migrations:
  project: ./Migrations   # path to your SDK project
  sdk: csharp             # csharp | python | any custom value
  table_case: snake       # snake (default) | camel | pascal
```

**2. Generate a migration:**

```bash
flowgrate make CreateUsersTable
```

**3. Apply migrations:**

```bash
flowgrate up
```

## Commands

| Command | Description |
|---------|-------------|
| `flowgrate init` | Generate a `flowgrate.yml` config file |
| `flowgrate make <Name>` | Generate a migration file with a timestamp prefix |
| `flowgrate up` | Apply all pending migrations |
| `flowgrate down` | Roll back the last migration |
| `flowgrate status` | Show applied and pending migrations |
| `flowgrate fresh` | Drop all tables and re-run all migrations |
| `flowgrate squash` | Dump current schema to `schema/{driver}-schema.sql` |
| `flowgrate help` | Show help |

### Options

```
up / down / status / fresh / squash:
  --db=DSN         Database DSN (overrides config)
  --config=FILE    Path to config file (default: flowgrate.yml)

up:
  --step=N         Apply only N migrations

down:
  --step=N         Roll back N migrations (default: 1)

fresh:
  --force          Skip confirmation prompt

squash:
  --prune          Delete all migration files after dumping
```

## Migration naming

The name you pass to `make` determines the generated template:

| Name | Template |
|------|----------|
| `CreateUsersTable` | `Schema.Create` |
| `AddStatusToUsers` | `Schema.Table` + `AddColumn` |
| `ChangeEmailInUsers` | `Schema.Table` + `ChangeColumn` |
| `DropAvatarFromUsers` | `Schema.Table` + `DropColumn` |
| `DropPostsTable` | `Schema.DropIfExists` |

Table and column names in generated files follow `table_case` from config:

| Value | Example | Common in |
|-------|---------|-----------|
| `snake` (default) | `user_profiles` | Python, Ruby, Go |
| `camel` | `userProfiles` | JavaScript, TypeScript |
| `pascal` | `UserProfiles` | C#, Java |

## Config reference

```yaml
database:
  url: postgres://user:pass@localhost/mydb  # or mysql:// | sqlite:///

migrations:
  project: ./Migrations   # where migration files live
  sdk: csharp             # csharp | python | any custom value
  run: dotnet run --project ./Migrations  # explicit invoke command (optional for built-in SDKs)
  table_case: snake       # snake (default) | camel | pascal

  # Custom / third-party SDK options:
  stubs: ./my-stubs       # directory with .tmpl files for custom make templates
  file_ext: .rb           # file extension for generated migration files
```

If `run` is not set, the CLI uses built-in defaults for `csharp` and `python`. For any other SDK value, `run` is required.

## Custom SDKs

Flowgrate supports any SDK that writes JSON manifests to stdout. See the [manifest spec](https://github.com/flowgrate/spec).

```yaml
migrations:
  sdk: ruby
  run: bundle exec ruby migrations/runner.rb
  table_case: snake
```

When `sdk` is set to an unknown value, `flowgrate make` generates a `.migration` JSON skeleton showing the expected output format. To use your own templates instead, point `stubs` at a directory with `.tmpl` files:

```
my-stubs/
  create.tmpl
  drop_table.tmpl
  add_column.tmpl
  change_column.tmpl
  drop_column.tmpl
  blank.tmpl
```

Templates use Go's `text/template` syntax with variables: `{{.ClassName}}`, `{{.Table}}`, `{{.Column}}`, `{{.Version}}`.

## squash

`flowgrate squash` dumps the current database schema (DDL + applied migration history) into a single SQL file. On the next `flowgrate up` against an **empty** database, the dump is loaded automatically — useful for onboarding new developers or CI environments.

```bash
flowgrate squash           # dump to schema/{driver}-schema.sql
flowgrate squash --prune   # dump and delete all migration files
```

The dump file should be committed to version control. Requires the native CLI client on the host:

| Database   | Dump tool      | Load tool  |
|------------|----------------|------------|
| PostgreSQL | `pg_dump`      | `psql`     |
| MySQL      | `mysqldump`    | `mysql`    |
| MariaDB    | `mariadb-dump` | `mariadb`  |
| SQLite     | `sqlite3`      | `sqlite3`  |

## Supported databases

| Database   | Migrations | squash |
|------------|------------|--------|
| PostgreSQL | ✅         | ✅     |
| MySQL      | ✅         | ✅     |
| MariaDB    | ✅         | ✅     |
| SQLite     | ✅         | ✅     |

## Language SDKs

| Language | Repository | Status |
|----------|------------|--------|
| C#       | [flowgrate/dotnet](https://github.com/flowgrate/dotnet) | ✅ |
| Python   | [flowgrate/python](https://github.com/flowgrate/python) | ✅ |

Want to build an SDK for another language? See the [manifest spec](https://github.com/flowgrate/spec).

## Docker usage

```bash
# Run SDK in Docker, CLI on host
docker compose exec sdk dotnet run --project /migrations | ./flowgrate up
```

## schema_migrations table

```sql
CREATE TABLE schema_migrations (
    id         SERIAL PRIMARY KEY,
    migration  VARCHAR(255) NOT NULL UNIQUE,
    batch      INTEGER NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```
