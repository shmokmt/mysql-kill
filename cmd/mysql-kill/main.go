package main

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/shmokmt/mysql-kill"
)

// main parses arguments and runs the CLI.
func main() {
	var cli mysqlkill.CLI
	ctx := kong.Parse(&cli,
		kong.Name("mysql-kill"),
		kong.Description("Kill a MySQL query/connection by process ID (pt-kill-inspired flags)."),
	)

	if err := mysqlkill.Run(context.Background(), &cli, ctx.Command()); err != nil {
		ctx.Fatalf("%v", err)
	}
}
