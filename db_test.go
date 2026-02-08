package mysqlkill

import (
	"database/sql"
	"testing"
)

func TestIsReaderFromValues(t *testing.T) {
	cases := []struct {
		name           string
		innodbReadOnly sql.NullInt64
		readOnly       sql.NullInt64
		expectedReader bool
	}{
		{
			name:           "innodb_read_only=1",
			innodbReadOnly: sql.NullInt64{Int64: 1, Valid: true},
			readOnly:       sql.NullInt64{Int64: 0, Valid: true},
			expectedReader: true,
		},
		{
			name:           "read_only=1",
			innodbReadOnly: sql.NullInt64{Int64: 0, Valid: true},
			readOnly:       sql.NullInt64{Int64: 1, Valid: true},
			expectedReader: true,
		},
		{
			name:           "both_zero",
			innodbReadOnly: sql.NullInt64{Int64: 0, Valid: true},
			readOnly:       sql.NullInt64{Int64: 0, Valid: true},
			expectedReader: false,
		},
		{
			name:           "nulls",
			innodbReadOnly: sql.NullInt64{Valid: false},
			readOnly:       sql.NullInt64{Valid: false},
			expectedReader: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isReaderFromValues(tc.innodbReadOnly, tc.readOnly)
			if got != tc.expectedReader {
				t.Fatalf("got %v want %v", got, tc.expectedReader)
			}
		})
	}
}
