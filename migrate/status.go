package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const showReplicaStatus = "SHOW REPLICA STATUS"

// ReadReplicaStatus reads one SHOW REPLICA STATUS row.
// Present is false when the server is not a replica.
func ReadReplicaStatus(ctx context.Context, db *sql.DB) (ReplicaStatus, error) {
	rows, err := db.QueryContext(ctx, showReplicaStatus)
	if err != nil {
		return ReplicaStatus{}, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return ReplicaStatus{}, err
	}
	if !rows.Next() {
		return ReplicaStatus{}, rows.Err()
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return ReplicaStatus{}, err
	}
	need := []string{
		"Replica_IO_Running",
		"Replica_SQL_Running",
		"Executed_Gtid_Set",
		"Retrieved_Gtid_Set",
		"Seconds_Behind_Source",
	}
	got := map[string]any{}
	for _, name := range need {
		v, ok := columnValue(cols, vals, name)
		if !ok {
			return ReplicaStatus{}, fmt.Errorf("status: SHOW REPLICA STATUS has no %s column", name)
		}
		got[name] = v
	}
	return ReplicaStatus{
		Present:       true,
		IORunning:     scalarString(got["Replica_IO_Running"]),
		SQLRunning:    scalarString(got["Replica_SQL_Running"]),
		ExecutedGTID:  scalarString(got["Executed_Gtid_Set"]),
		RetrievedGTID: scalarString(got["Retrieved_Gtid_Set"]),
		SecondsBehind: parseBehind(got["Seconds_Behind_Source"]),
	}, rows.Err()
}

func columnValue(cols []string, vals []any, name string) (any, bool) {
	for i, col := range cols {
		if strings.EqualFold(col, name) {
			return vals[i], true
		}
	}
	return nil, false
}

func scalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}

func parseBehind(v any) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	switch t := v.(type) {
	case int64:
		return sql.NullInt64{Int64: t, Valid: true}
	case int32:
		return sql.NullInt64{Int64: int64(t), Valid: true}
	case uint64:
		return sql.NullInt64{Int64: int64(t), Valid: true}
	case []byte:
		return parseBehindString(string(t))
	case string:
		return parseBehindString(t)
	default:
		return parseBehindString(fmt.Sprint(t))
	}
}

func parseBehindString(s string) sql.NullInt64 {
	if strings.TrimSpace(s) == "" {
		return sql.NullInt64{}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: n, Valid: true}
}

// FormatStatus writes the five replica fields, or a line when the server is not a replica.
func FormatStatus(w io.Writer, s ReplicaStatus) error {
	if !s.Present {
		_, err := io.WriteString(w, "server is not a replica\n")
		return err
	}
	behind := "NULL"
	if s.SecondsBehind.Valid {
		behind = strconv.FormatInt(s.SecondsBehind.Int64, 10)
	}
	_, err := fmt.Fprintf(w, "Replica_IO_Running: %s\nReplica_SQL_Running: %s\nExecuted_Gtid_Set: %s\nRetrieved_Gtid_Set: %s\nSeconds_Behind_Source: %s\n",
		s.IORunning, s.SQLRunning, s.ExecutedGTID, s.RetrievedGTID, behind)
	return err
}
