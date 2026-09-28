package migrate

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Cutover waits until the replica has caught up, then runs STOP REPLICA.
// It does not connect to MySQL to make the source read-only.
func Cutover(ctx context.Context, dstURL string, timeout, interval time.Duration, stdout io.Writer) error {
	if timeout <= 0 {
		return fmt.Errorf("cutover: timeout must be greater than zero")
	}
	if interval <= 0 {
		return fmt.Errorf("cutover: interval must be greater than zero")
	}
	if _, err := io.WriteString(stdout, "Make MySQL read-only before cutover. This command does not change MySQL.\n"); err != nil {
		return err
	}
	_, db, err := openMySQL(ctx, dstURL)
	if err != nil {
		return err
	}
	defer db.Close()
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		st, err := ReadReplicaStatus(ctx, db)
		if err != nil {
			return err
		}
		if !st.Present {
			return fmt.Errorf("cutover: server is not a replica")
		}
		if ReadyToCutOver(st) {
			break
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("cutover: replica did not catch up within %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	if _, err := db.ExecContext(ctx, "STOP REPLICA"); err != nil {
		return fmt.Errorf("cutover: STOP REPLICA failed: %w", err)
	}
	_, err = io.WriteString(stdout, "replica stopped\n")
	return err
}
