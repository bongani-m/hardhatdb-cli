package migrate

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/xo/dburl"
)

// Replicate points HardhatDB at a MySQL binlog and starts the replica.
// The source URL supplies the host, port, user, and password. It is not read for rows.
func Replicate(ctx context.Context, srcURL, dstURL string, databases []string, stdout io.Writer) error {
	if len(databases) == 0 {
		return fmt.Errorf("replicate: name at least one database")
	}
	for _, name := range databases {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("replicate: database name is empty")
		}
		if isSystemDatabase(name) {
			return fmt.Errorf("replicate: %s is a system database", name)
		}
	}
	srcU, err := dburl.Parse(srcURL)
	if err != nil {
		return err
	}
	if err := requireMySQL(srcU); err != nil {
		return err
	}
	host, port, user, password, err := replicationSource(srcU)
	if err != nil {
		return err
	}
	if SourceUsesTLS(srcU.Query()) {
		return CheckReplicate(ReplicateGuard{SourceTLS: true})
	}
	_, dst, err := openMySQL(ctx, dstURL)
	if err != nil {
		return err
	}
	defer dst.Close()
	var withTables []string
	for _, name := range databases {
		n, err := queryCount(ctx, dst, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE'`, name)
		if err != nil {
			return fmt.Errorf("replicate: list tables in %s: %w", name, err)
		}
		if n > 0 {
			withTables = append(withTables, name)
		}
	}
	status, err := ReadReplicaStatus(ctx, dst)
	if err != nil {
		return fmt.Errorf("replicate: %w", err)
	}
	guard := ReplicateGuard{DatabasesWithTables: withTables}
	if status.Present {
		guard.ExecutedGTID = status.ExecutedGTID
	}
	if err := CheckReplicate(guard); err != nil {
		return err
	}
	if _, err := dst.ExecContext(ctx, FilterWildDo(databases)); err != nil {
		return fmt.Errorf("replicate: CHANGE REPLICATION FILTER failed: %w", err)
	}
	if _, err := dst.ExecContext(ctx, ChangeSource(host, port, user, password)); err != nil {
		return fmt.Errorf("replicate: CHANGE REPLICATION SOURCE TO failed: %w", err)
	}
	if _, err := dst.ExecContext(ctx, "START REPLICA"); err != nil {
		return fmt.Errorf("replicate: START REPLICA failed: %w", err)
	}
	_, err = fmt.Fprintf(stdout, "replication started from %s:%d as %s for %s\n", host, port, user, strings.Join(databases, ", "))
	return err
}
