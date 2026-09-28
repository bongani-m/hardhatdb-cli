package migrate

import (
	"database/sql"
	"testing"

	"github.com/xo/dburl"
)

func TestCheckReplicate(t *testing.T) {
	tests := []struct {
		name    string
		guard   ReplicateGuard
		wantErr bool
	}{
		{
			name:  "empty destination",
			guard: ReplicateGuard{},
		},
		{
			name:    "tables present",
			guard:   ReplicateGuard{DatabasesWithTables: []string{"appdb"}},
			wantErr: true,
		},
		{
			name:    "gtid set present",
			guard:   ReplicateGuard{ExecutedGTID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:1-9"},
			wantErr: true,
		},
		{
			name:    "tls in the source url",
			guard:   ReplicateGuard{SourceTLS: true},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := CheckReplicate(test.guard)
			if test.wantErr && err == nil {
				t.Fatal("expected an error")
			}
			if !test.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSourceUsesTLS(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{"mysql://repl:secret@mysql.example:3306/appdb", false},
		{"mysql://repl:secret@mysql.example:3306/appdb?tls=false", false},
		{"mysql://repl:secret@mysql.example:3306/appdb?tls=0", false},
		{"mysql://repl:secret@mysql.example:3306/appdb?tls=skip-verify", true},
		{"mysql://repl:secret@mysql.example:3306/appdb?tls=true", true},
	}
	for _, test := range tests {
		u, err := dburl.Parse(test.raw)
		if err != nil {
			t.Fatalf("parse %s: %v", test.raw, err)
		}
		if got := SourceUsesTLS(u.Query()); got != test.want {
			t.Fatalf("%s: tls=%v want %v (query %q)", test.raw, got, test.want, u.RawQuery)
		}
	}
}

func TestReadyToCutOver(t *testing.T) {
	match := ReplicaStatus{
		Present:       true,
		IORunning:     "Yes",
		SQLRunning:    "Yes",
		ExecutedGTID:  "uuid:1-4",
		RetrievedGTID: "uuid:1-4",
		SecondsBehind: sql.NullInt64{Int64: 0, Valid: true},
	}
	tests := []struct {
		name string
		in   ReplicaStatus
		want bool
	}{
		{name: "matching gtid and lag 0", in: match, want: true},
		{name: "gtid mismatch", in: func() ReplicaStatus {
			s := match
			s.RetrievedGTID = "uuid:1-5"
			return s
		}(), want: false},
		{name: "stopped io thread", in: func() ReplicaStatus {
			s := match
			s.IORunning = "No"
			return s
		}(), want: false},
		{name: "lag above 0", in: func() ReplicaStatus {
			s := match
			s.SecondsBehind = sql.NullInt64{Int64: 2, Valid: true}
			return s
		}(), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ReadyToCutOver(test.in); got != test.want {
				t.Fatalf("got %v want %v", got, test.want)
			}
		})
	}
}

func TestChangeSourceQuotesPassword(t *testing.T) {
	got := ChangeSource("mysql.example", 3306, "repl", "s'ecret")
	want := "CHANGE REPLICATION SOURCE TO SOURCE_HOST = 'mysql.example', SOURCE_PORT = 3306, SOURCE_USER = 'repl', SOURCE_PASSWORD = 's\\'ecret', SOURCE_AUTO_POSITION = 1"
	if got != want {
		t.Fatalf("got %s", got)
	}
	filter := FilterWildDo([]string{"appdb", "other"})
	wantFilter := "CHANGE REPLICATION FILTER REPLICATE_WILD_DO_TABLE = ('appdb.%', 'other.%')"
	if filter != wantFilter {
		t.Fatalf("got %s", filter)
	}
}
