# mysql-kill

Kill a MySQL query/connection by process ID with pt-kill-inspired flags.

## Features

- Single-argument CLI: `mysql-kill <id>`
- pt-kill-compatible kill flags: `--kill` / `--kill-query`
- Default `--dry-run` (safe by default)
- Auto-detect Amazon RDS / Aurora MySQL and use `mysql.rds_kill*` procedures
- Environment variables take precedence over flags
- Optional SSH tunnel (bastion) with strict host key checking by default

## Usage

```bash
# List running queries
mysql-kill list

# List with filters (match is regex)
mysql-kill list --user app --min-time 10 --match "SELECT"

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
```

## Connection configuration

Configuration file: `~/.config/mysql-kill/config.toml`

Configuration precedence is:

1. `~/.config/mysql-kill/config.toml`
2. Environment variables
3. CLI flags

If `MYSQL_DSN` is set, it takes precedence and other MySQL settings are ignored.

Supported environment variables:

- `MYSQL_DSN`
- `MYSQL_HOST` (default: `127.0.0.1`)
- `MYSQL_PORT` (default: `3306`)
- `MYSQL_USER` (default: `root`)
- `MYSQL_PASSWORD`
- `MYSQL_DB`
- `MYSQL_SOCKET`
- `MYSQL_TLS`

Flag equivalents are also available (see `--help`).

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

If `SSH_HOST` (or `--ssh-host`) is provided, the tool opens a local SSH tunnel and connects to the target DB host/port through it.
Strict host key checking is enabled by default.

Supported environment variables:

- `SSH_HOST`
- `SSH_PORT` (default: `22`)
- `SSH_USER` (default: `$USER`)
- `SSH_KEY` (private key path)
- `SSH_KNOWN_HOSTS` (default: `~/.ssh/known_hosts`)
- `SSH_NO_STRICT_HOST_KEY` (set to `true` to disable strict checking)

Example:

```bash
export SSH_HOST=bastion.example.com
export SSH_USER=ec2-user
export MYSQL_HOST=internal-db.example.com
export MYSQL_PORT=3306

mysql-kill 123 --kill-query
```

Notes:

- The DB host/port are still configured via `MYSQL_HOST`/`MYSQL_PORT` (or flags), even when tunneling.
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
