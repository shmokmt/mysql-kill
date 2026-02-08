package mysqlkill

import (
	"context"
	"errors"
)

// CLI defines the top-level command structure for mysql-kill.
type CLI struct {
	DSN      string `help:"MySQL DSN (env: MYSQL_DSN)."`
	Host     string `name:"mysql-host" help:"MySQL host (env: MYSQL_HOST)."`
	Port     int    `help:"MySQL port (env: MYSQL_PORT)."`
	User     string `name:"mysql-user" help:"MySQL user (env: MYSQL_USER)."`
	Password string `help:"MySQL password (env: MYSQL_PASSWORD)."`
	DB       string `name:"mysql-db" help:"MySQL database (env: MYSQL_DB)."`
	Socket   string `help:"MySQL unix socket (env: MYSQL_SOCKET)."`
	TLS      string `help:"MySQL TLS config name (env: MYSQL_TLS)."`

	SSHHost            string `help:"SSH bastion host (env: SSH_HOST)."`
	SSHPort            int    `help:"SSH bastion port (env: SSH_PORT)."`
	SSHUser            string `help:"SSH bastion user (env: SSH_USER)."`
	SSHKey             string `help:"SSH private key file path (env: SSH_KEY)."`
	SSHKnownHosts      string `help:"known_hosts path for strict checking (env: SSH_KNOWN_HOSTS)."`
	SSHNoStrictHostKey bool   `help:"Disable strict host key checking (env: SSH_NO_STRICT_HOST_KEY)."`

	AllowWriter bool `help:"Allow connecting to writer/primary (default: reader only)."`

	Kill *KillCmd `cmd:"" help:"Kill a query or connection by process ID."`
	List *ListCmd `cmd:"" help:"List running queries (from processlist)."`
}

// KillCmd represents the kill subcommand.
type KillCmd struct {
	QueryID   int64 `arg:"" name:"id" help:"MySQL process (query) ID to target."`
	Kill      bool  `help:"Kill the connection (pt-kill-inspired --kill)."`
	KillQuery bool  `help:"Kill only the running query (pt-kill-inspired --kill-query)."`
	DryRun    bool  `help:"Print the SQL/CALL without executing."`
}

// ListCmd represents the list subcommand.
type ListCmd struct {
	Match string `help:"Filter by SQL substring (INFO)."`
}

// Run executes the selected subcommand.
func Run(ctx context.Context, cli *CLI) error {
	if cli.Kill != nil {
		return runKill(ctx, cli, cli.Kill)
	}

	if cli.List != nil {
		return runList(ctx, cli, cli.List)
	}

	return errors.New("command required: use 'kill' or 'list'")
}
