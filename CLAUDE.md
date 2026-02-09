# CLAUDE.md

## Project Overview

**mysql-kill** is a Go CLI tool that kills MySQL queries/connections by process ID, inspired by `pt-kill`. It auto-detects Amazon RDS/Aurora MySQL and uses the appropriate `mysql.rds_kill*` stored procedures. Supports SSH tunneling through a bastion host.

- **Module:** `github.com/shmokmt/mysql-kill`
- **Go version:** 1.24.0 (toolchain 1.24.4)
- **CLI framework:** [Kong](https://github.com/alecthomas/kong)
- **Entry point:** `cmd/mysql-kill/main.go`

## Repository Structure

```
cmd/mysql-kill/main.go   # Binary entry point: parses args, calls mysqlkill.Run()
cli.go                   # CLI struct definitions (Kong framework)
config.go                # Multi-source config resolution (file > env > flags)
db.go                    # Database connection, RDS/Aurora detection, reader/writer checks
kill.go                  # Kill command implementation (standard & RDS)
list.go                  # List command (PROCESSLIST query with regex filtering)
ssh.go                   # SSH tunnel establishment and lifecycle
compose.yaml             # Docker Compose for integration test MySQL instance
.goreleaser.yml          # Cross-platform release builds
.github/workflows/       # CI: go-test.yml, goreleaser.yml, tagpr.yml
```

## Build & Run

```bash
# Build
go build ./cmd/mysql-kill

# Install
go install github.com/shmokmt/mysql-kill/cmd/mysql-kill@latest
```

## Testing

### Unit tests (CI default)

```bash
go test ./...
```

### Integration tests (require Docker MySQL)

```bash
docker compose up -d
go test -tags=integration ./...
```

Integration tests use build tag `//go:build integration` and connect to MySQL on port 3307 by default. Test connection values are configured via environment variables: `MYSQL_TEST_HOST` (127.0.0.1), `MYSQL_TEST_PORT` (3307), `MYSQL_TEST_USER` (root), `MYSQL_TEST_PASSWORD` (testpass), `MYSQL_TEST_DB` (testdb).

## CI/CD

- **go-test.yml** — Runs `go test ./...` on every push and PR
- **goreleaser.yml** — Builds cross-platform binaries on version tags (`v*.*.*`)
- **tagpr.yml** — Auto-generates release PRs with version bumps on main

## Architecture & Key Patterns

### Execution flow

`main.go` → `kong.Parse(&cli)` → `mysqlkill.Run(ctx, &cli)` → dispatches to `runKill()` or `runList()`

### Configuration precedence

1. Config file (TOML, searched in XDG_CONFIG_HOME → os.UserConfigDir → ~/.config)
2. Environment variables (`MYSQL_DSN` overrides all other MySQL settings)
3. CLI flags

### RDS/Aurora detection

The tool queries `version_comment` and checks for the `mysql.rds_kill` stored procedure. If detected as RDS, it uses `CALL mysql.rds_kill(id)` / `CALL mysql.rds_kill_query(id)` instead of standard `KILL` statements.

### Safety defaults

- Connections to writer/primary instances are blocked by default (use `--allow-writer` to override)
- SSH strict host key checking is enabled by default
- `--kill` and `--kill-query` are mutually exclusive; one is required for the kill command

## Code Conventions

- **Package name:** `mysqlkill` (root package, binary in `cmd/mysql-kill/`)
- **Error handling:** Errors wrapped with `fmt.Errorf("context: %w", err)`
- **Testing:** Standard `testing` package, table-driven tests with `t.Run()`, no external test framework
- **Interfaces:** Used for dependency injection (e.g., `Execer` interface in db.go)
- **Resource cleanup:** `defer` with `sync.Once` for idempotent Close operations
- **No linter config:** Follow standard `gofmt` formatting
- **No Makefile:** Use `go` toolchain directly
