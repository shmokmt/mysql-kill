# mysql-kill

Kill a MySQL query/connection by process ID with pt-kill-inspired flags.

## Features

 - Subcommands: `kill` and `list`
 - pt-kill-inspired kill flags: `--kill` / `--kill-query`
- Auto-detect Amazon RDS / Aurora MySQL and use `mysql.rds_kill*` procedures
- Config file (TOML) with minimal CLI flags
- Optional SSH tunnel (bastion) with strict host key checking by default

## Usage

```bash
# List running queries
mysql-kill list

# List with regex match
mysql-kill list --match "SELECT"

# List Redash queries (match query comment regex)
mysql-kill list --match "/\\* redash"

# Kill a specific Redash query by ID
mysql-kill kill 123 --kill-query

# Allow writer/primary (default is reader-only)
mysql-kill list --allow-writer

# Show the SQL/CALL to be executed
mysql-kill kill 123 --kill-query --dry-run

# Kill only the running query (standard MySQL or RDS/Aurora detected)
mysql-kill kill 123 --kill-query

# Kill the entire connection
mysql-kill kill 123 --kill

# Allow writer/primary (default is reader-only)
mysql-kill kill 123 --allow-writer --kill-query

# Explicitly enable dry-run
mysql-kill kill 123 --kill --dry-run

# Use a specific config file
mysql-kill -c ~/.config/mysql-kill/staging.toml list

# One-shot connection via DSN (overrides config file)
mysql-kill --dsn "user:pass@tcp(host:3306)/db" list
```

## Install (Go)

```bash
go install github.com/shmokmt/mysql-kill/cmd/mysql-kill@latest
```

## Connection configuration

All connection settings are managed via a TOML config file. CLI flags are limited to:

| Flag | Description |
|------|-------------|
| `--dsn` | MySQL DSN (overrides config file) |
| `--allow-writer` | Allow connecting to writer/primary |
| `-c`, `--config` | Path to config file |

### Config file search order

If `--config` is not specified, the first found file is used:

1. `$XDG_CONFIG_HOME/mysql-kill/config.toml`
2. `os.UserConfigDir()/mysql-kill/config.toml`
3. `~/.config/mysql-kill/config.toml`

### Configuration precedence

1. CLI flags (`--dsn`, `--allow-writer`)
2. Config file

### config.toml example

```toml
[mysql-kill]
allow_writer = false

[mysql]
host = "127.0.0.1"
port = 3306
user = "root"
password = "secret"
db = "testdb"
tls = "custom"

[ssh]
host = "bastion.example.com"
port = 22
user = "ec2-user"
key = "~/.ssh/id_rsa"
known_hosts = "~/.ssh/known_hosts"
no_strict_host_key = false
```

## Auto-detect RDS/Aurora

The tool connects to the database and determines whether it is Amazon RDS or Aurora MySQL. If it is, it will use:

- `CALL mysql.rds_kill(<id>)`
- `CALL mysql.rds_kill_query(<id>)`

Otherwise it will use standard MySQL statements:

- `KILL <id>`
- `KILL QUERY <id>`

If auto-detection fails, the command exits with an error instead of falling back to standard `KILL`.

## SSH tunnel (bastion)

If `ssh.host` is set in the config file, the tool opens a local SSH tunnel and connects to the target DB host/port through it.
Strict host key checking is enabled by default.

Example config:

```toml
[mysql]
host = "internal-db.example.com"
port = 3306

[ssh]
host = "bastion.example.com"
user = "ec2-user"
key = "~/.ssh/id_rsa"
```

Notes:

- The DB host/port are configured via `[mysql]` section, even when tunneling.
- `--dry-run` still connects in order to auto-detect RDS/Aurora, so the SSH tunnel will be used.

## Notes

- `--kill` and `--kill-query` are mutually exclusive.
- `--kill` or `--kill-query` is required for the kill command.
- By default, the tool requires the target to be a reader (read-only). Use `--allow-writer` to allow writer/primary connections.

## Integration tests (Docker)

This project uses real MySQL via Docker for integration tests (no mocks).

```bash
# Start MySQL
docker compose up -d

# Run integration tests
go test -tags=integration ./...
```

Default test connection values (override with env vars if needed):

- `MYSQL_TEST_HOST=127.0.0.1`
- `MYSQL_TEST_PORT=3307`
- `MYSQL_TEST_USER=root`
- `MYSQL_TEST_PASSWORD=testpass`
- `MYSQL_TEST_DB=testdb`
