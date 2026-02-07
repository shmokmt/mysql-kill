package mysqlkill

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
)

// runList executes the list command.
func runList(ctx context.Context, cli *CLI, cmd *ListCmd) error {
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

	if err := enforceReader(ctx, db, cli.AllowWriter); err != nil {
		return err
	}

	return listProcess(ctx, db, cmd)
}

// listProcess queries and prints the processlist.
func listProcess(ctx context.Context, db *sql.DB, cmd *ListCmd) error {
	query, args := buildProcessListQuery(cmd)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query processlist: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tUSER\tHOST\tDB\tCOMMAND\tTIME\tSTATE\tINFO")

	for rows.Next() {
		var (
			id      int64
			user    sql.NullString
			host    sql.NullString
			db      sql.NullString
			command sql.NullString
			timeSec sql.NullInt64
			state   sql.NullString
			info    sql.NullString
		)
		if err := rows.Scan(&id, &user, &host, &db, &command, &timeSec, &state, &info); err != nil {
			return fmt.Errorf("scan processlist: %w", err)
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			id,
			nullString(user),
			nullString(host),
			nullString(db),
			nullString(command),
			nullInt(timeSec),
			nullString(state),
			nullString(info),
		)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows: %w", err)
	}

	return tw.Flush()
}

// buildProcessListQuery builds the processlist query and args.
func buildProcessListQuery(cmd *ListCmd) (string, []any) {
	base := `SELECT ID, USER, HOST, DB, COMMAND, TIME, STATE, INFO FROM information_schema.processlist`
	var where []string
	var args []any

	if cmd.User != "" {
		where = append(where, "USER = ?")
		args = append(args, cmd.User)
	}
	if cmd.DB != "" {
		where = append(where, "DB = ?")
		args = append(args, cmd.DB)
	}
	if cmd.Host != "" {
		where = append(where, "HOST LIKE ?")
		args = append(args, "%"+cmd.Host+"%")
	}
	if cmd.Command != "" {
		where = append(where, "COMMAND = ?")
		args = append(args, cmd.Command)
	}
	if cmd.State != "" {
		where = append(where, "STATE LIKE ?")
		args = append(args, "%"+cmd.State+"%")
	}
	if cmd.Match != "" {
		where = append(where, "INFO REGEXP ?")
		args = append(args, cmd.Match)
	}
	if cmd.MinTime > 0 {
		where = append(where, "TIME >= ?")
		args = append(args, cmd.MinTime)
	}

	query := base
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY TIME DESC"
	if cmd.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", cmd.Limit)
	}

	return query, args
}

// nullString converts sql.NullString to a plain string.
func nullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

// nullInt converts sql.NullInt64 to a plain string.
func nullInt(v sql.NullInt64) string {
	if v.Valid {
		return strconv.FormatInt(v.Int64, 10)
	}
	return ""
}
