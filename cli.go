package mysqlkill

import (
	"context"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
)

// CLI defines the top-level command structure for mysql-kill.
type CLI struct {
	DSN         string `help:"MySQL DSN (overrides config file)."`
	AllowWriter bool   `help:"Allow connecting to writer/primary (default: reader only)."`
	Config      string `short:"c" help:"Path to config file (default: auto-detect)."`

	Version kong.VersionFlag `name:"version" help:"Print version information and quit."`

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
	Match string `help:"Filter by SQL regex (INFO)."`
}

// Run executes the selected subcommand.
func Run(ctx context.Context, cli *CLI, command string) error {
	switch {
	case strings.HasPrefix(command, "kill"):
		return runKill(ctx, cli, cli.Kill)
	case command == "list":
		return runList(ctx, cli, cli.List)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}
