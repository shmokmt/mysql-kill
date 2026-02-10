package main

import (
	"context"
	"os"

	"github.com/alecthomas/kong"
	"github.com/shmokmt/mysql-kill"
)

// version is set at build time via ldflags.
var version = "dev"

// main parses arguments and runs the CLI.
func main() {
	// Show help when no arguments are given.
	if len(os.Args) <= 1 {
		os.Args = append(os.Args, "--help")
	}

	var cli mysqlkill.CLI
	kongCtx := kong.Parse(&cli,
		kong.Name("mysql-kill"),
		kong.Description("Kill a MySQL query/connection by process ID (pt-kill-inspired flags)."),
		kong.Vars{"version": version},
	)

	if err := mysqlkill.Run(context.Background(), &cli, kongCtx.Command()); err != nil {
		kongCtx.Fatalf("%v", err)
	}
}
