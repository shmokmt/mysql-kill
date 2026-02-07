package mysqlkill

import (
	"reflect"
	"testing"
)

func TestBuildProcesslistQuery(t *testing.T) {
	cmd := &ListCmd{
		User:    "app",
		DB:      "db1",
		Host:    "10.0.",
		Command: "Query",
		State:   "Locked",
		Match:   "SELECT",
		MinTime: 5,
		Limit:   50,
	}

	gotQuery, gotArgs := buildProcessListQuery(cmd)
	wantQuery := "SELECT ID, USER, HOST, DB, COMMAND, TIME, STATE, INFO FROM information_schema.processlist WHERE USER = ? AND DB = ? AND HOST LIKE ? AND COMMAND = ? AND STATE LIKE ? AND INFO REGEXP ? AND TIME >= ? ORDER BY TIME DESC LIMIT 50"
	wantArgs := []any{"app", "db1", "%10.0.%", "Query", "%Locked%", "SELECT", 5}

	if gotQuery != wantQuery {
		t.Fatalf("query mismatch:\n%s\n!=\n%s", gotQuery, wantQuery)
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args mismatch: %#v != %#v", gotArgs, wantArgs)
	}
}

func TestBuildProcesslistQueryNoFilters(t *testing.T) {
	cmd := &ListCmd{}
	gotQuery, gotArgs := buildProcessListQuery(cmd)
	wantQuery := "SELECT ID, USER, HOST, DB, COMMAND, TIME, STATE, INFO FROM information_schema.processlist ORDER BY TIME DESC"

	if gotQuery != wantQuery {
		t.Fatalf("query mismatch: %s != %s", gotQuery, wantQuery)
	}
	if len(gotArgs) != 0 {
		t.Fatalf("expected no args, got %#v", gotArgs)
	}
}
