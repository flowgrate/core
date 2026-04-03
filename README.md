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
```

### Build from source

```bash
git clone https://github.com/flowgrate/core
cd core
go build -o flowgrate .
```

## Quick start

**1. Create `flowgrate.yml`:**

```yaml
database:
  url: postgres://user:pass@localhost/mydb

migrations:
  project: ./Migrations   # path to your SDK project
  sdk: csharp             # csharp | python
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

## Docker usage

```bash
# Run SDK in Docker, CLI on host
docker compose exec sdk dotnet run --project /migrations | ./flowgrate up
```

## squash

`flowgrate squash` dumps the current database schema (DDL + applied migration history) into a single SQL file. On the next `flowgrate up` against an **empty** database, the dump is loaded automatically instead of replaying all migrations from scratch — useful for onboarding new developers or setting up CI environments.

```bash
# Dump schema
flowgrate squash

# Dump and delete all migration files
flowgrate squash --prune
```

The dump is stored as `schema/{driver}-schema.sql` next to `flowgrate.yml` and should be committed to version control.

Requires the native CLI client to be installed on the host:

| Database   | Dump tool      | Load tool  |
|------------|----------------|------------|
| PostgreSQL | `pg_dump`      | `psql`     |
| MySQL      | `mysqldump`    | `mysql`    |
| MariaDB    | `mariadb-dump` | `mariadb`  |
| SQLite     | `sqlite3`      | `sqlite3`  |

## Supported databases

| Database   | Migrations | squash |
|------------|------------|--------|
| PostgreSQL | ✅ v1      | ✅ v1  |
| MySQL      | planned    | ✅ v1  |
| MariaDB    | planned    | ✅ v1  |
| SQLite     | planned    | ✅ v1  |

## Language SDKs

| Language | Repository | Status |
|----------|------------|--------|
| C#       | [flowgrate/dotnet](https://github.com/flowgrate/dotnet) | ✅ v1 |
| Python   | [flowgrate/python](https://github.com/flowgrate/python) | ✅ v1 |

Want to build an SDK for another language? See the [manifest spec](https://github.com/flowgrate/spec).

## schema_migrations table

```sql
CREATE TABLE schema_migrations (
    id         SERIAL PRIMARY KEY,
    migration  VARCHAR(255) NOT NULL UNIQUE,
    batch      INTEGER NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```
