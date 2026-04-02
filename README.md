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
| `flowgrate help` | Show help |

### Options

```
up / down / status / fresh:
  --db=DSN         PostgreSQL DSN (overrides config)
  --config=FILE    Path to config file (default: flowgrate.yml)

up:
  --step=N         Apply only N migrations

down:
  --step=N         Roll back N migrations (default: 1)

fresh:
  --force          Skip confirmation prompt
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

## Supported databases

| Database   | Status |
|------------|--------|
| PostgreSQL | ✅ v1  |
| MySQL      | planned |
| SQLite     | planned |

## Language SDKs

| Language | Repository | Status |
|----------|------------|--------|
| C#       | [flowgrate/dotnet](https://github.com/flowgrate/dotnet) | ✅ v1 |
| Python   | [flowgrate/python](https://github.com/flowgrate/python) | planned |

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
