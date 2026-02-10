package main

import (
	"context"
	"os"
	"runtime/debug"

	"github.com/alecthomas/kong"
	"github.com/shmokmt/mysql-kill"
)

func getVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.tag" {
			return s.Value
		}
	}
	return "dev"
}

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
		kong.Vars{"version": getVersion()},
	)

	if err := mysqlkill.Run(context.Background(), &cli, kongCtx.Command()); err != nil {
		kongCtx.Fatalf("%v", err)
	}
}
