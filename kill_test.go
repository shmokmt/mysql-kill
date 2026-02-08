package mysqlkill

import "testing"

func TestBuildKillSQL(t *testing.T) {
	cases := []struct {
		name      string
		rds       bool
		kill      bool
		killQuery bool
		id        int64
		want      string
	}{
		{name: "mysql kill", rds: false, kill: true, id: 10, want: "KILL 10"},
		{name: "mysql kill query", rds: false, killQuery: true, id: 11, want: "KILL QUERY 11"},
		{name: "rds kill", rds: true, kill: true, id: 12, want: "CALL mysql.rds_kill(12)"},
		{name: "rds kill query", rds: true, killQuery: true, id: 13, want: "CALL mysql.rds_kill_query(13)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildKillSQL(tc.rds, tc.kill, tc.killQuery, tc.id)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
