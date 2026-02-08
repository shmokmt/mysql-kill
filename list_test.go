package mysqlkill

import (
	"reflect"
	"testing"
)

func TestBuildProcesslistQuery(t *testing.T) {
	cmd := &ListCmd{
		Match: "SELECT",
	}

	gotQuery, gotArgs := buildProcessListQuery(cmd)
	wantQuery := "SELECT ID, USER, HOST, DB, COMMAND, TIME, STATE, INFO FROM information_schema.processlist WHERE INFO REGEXP ? ORDER BY TIME DESC"
	wantArgs := []any{"SELECT"}

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
