package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bongani-m/hardhatdb-cli/drivers"
)

func TestHardhatBackupQueryType(t *testing.T) {
	typ, query := drivers.QueryExecType("BACKUP TO '/var/backups/1'", "BACKUP TO '/var/backups/1'")
	if typ != "BACKUP" || !query {
		t.Fatalf("BACKUP type = %q query = %v", typ, query)
	}
	typ, query = drivers.QueryExecType("RESTORE BINLOG FROM '/var/backups/1/binlog' AFTER 12", "RESTORE BINLOG FROM '/var/backups/1/binlog' AFTER 12")
	if typ != "RESTORE BINLOG" || query {
		t.Fatalf("RESTORE BINLOG type = %q query = %v", typ, query)
	}
}

func TestRestoreRequiresFlags(t *testing.T) {
	err := New([]string{"hardhatdb-cli", "restore"}).ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "from") || !strings.Contains(err.Error(), "data") {
		t.Fatalf("missing flags: %v", err)
	}
	err = New([]string{"hardhatdb-cli", "restore", "--from", t.TempDir()}).ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "data") {
		t.Fatalf("missing --data: %v", err)
	}
}

func TestRestoreRefusesExistingDataDir(t *testing.T) {
	data := t.TempDir()
	err := New([]string{
		"hardhatdb-cli", "restore",
		"--from", filepath.Join(t.TempDir(), "backup"),
		"--data", data,
	}).ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing data dir: %v", err)
	}
	if _, statErr := os.Stat(data); statErr != nil {
		t.Fatal(statErr)
	}
}
