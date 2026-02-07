package mysqlkill

import (
	"context"
	"errors"
	"fmt"
)

// runKill executes the kill command.
func runKill(ctx context.Context, cli *CLI, cmd *KillCmd) error {
	if cmd.QueryID == 0 {
		return errors.New("query id is required")
	}

	if cmd.Kill && cmd.KillQuery {
		return errors.New("--kill and --kill-query are mutually exclusive")
	}

	if !cmd.Kill && !cmd.KillQuery {
		if !cmd.DryRun {
			return errors.New("no action specified: use --kill or --kill-query, or rely on default --dry-run")
		}
	}

	cfg := resolveConfig(cli)
	if cfg.DSN == "" {
		cfg.DSN = buildDSN(cfg)
	}
	if cfg.DSN == "" {
		return errors.New("connection info missing: provide MYSQL_DSN or host/user parameters")
	}

	db, tunnel, err := openDBWithTunnel(ctx, cfg, resolveSSHConfig(cli))
	if err != nil {
		return err
	}
	defer func() {
		if tunnel != nil {
			tunnel.Close()
		}
		_ = db.Close()
	}()

	isRDS, err := detectRDS(ctx, db)
	if err != nil {
		return err
	}

	if err := enforceReader(ctx, db, cli.AllowWriter); err != nil {
		return err
	}

	sqlText := buildKillSQL(isRDS, cmd.Kill, cmd.KillQuery, cmd.QueryID)

	if cmd.DryRun || (!cmd.Kill && !cmd.KillQuery) {
		fmt.Printf("DRY RUN: %s\n", sqlText)
		return nil
	}

	if isRDS {
		return execRDSKill(ctx, db, cmd.Kill, cmd.QueryID)
	}

	if _, err := db.ExecContext(ctx, sqlText); err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	fmt.Printf("OK: %s\n", sqlText)
	return nil
}

// buildKillSQL builds the kill statement or RDS stored procedure call.
func buildKillSQL(rds bool, kill bool, killQuery bool, id int64) string {
	if rds {
		if killQuery {
			return fmt.Sprintf("CALL mysql.rds_kill_query(%d)", id)
		}
		return fmt.Sprintf("CALL mysql.rds_kill(%d)", id)
	}

	if killQuery {
		return fmt.Sprintf("KILL QUERY %d", id)
	}
	return fmt.Sprintf("KILL %d", id)
}

// execRDSKill executes the RDS kill stored procedure.
func execRDSKill(ctx context.Context, db Execer, kill bool, id int64) error {
	proc := "mysql.rds_kill"
	if !kill {
		proc = "mysql.rds_kill_query"
	}
	if _, err := db.ExecContext(ctx, "CALL "+proc+"(?)", id); err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	fmt.Printf("OK: CALL %s(%d)\n", proc, id)
	return nil
}
