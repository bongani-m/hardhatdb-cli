package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/bongani-m/hardhatdb-cli/drivers"
	"github.com/xo/dburl"
)

// Load copies schema and rows from MySQL into HardhatDB.
// Views, triggers, routines, and events are named on stderr and left behind.
func Load(ctx context.Context, srcURL, dstURL string, databases []string, stdout, stderr io.Writer) error {
	if len(databases) == 0 {
		return fmt.Errorf("load: name at least one database")
	}
	for _, name := range databases {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("load: database name is empty")
		}
		if isSystemDatabase(name) {
			return fmt.Errorf("load: %s is a system database", name)
		}
	}
	srcU, src, err := openMySQL(ctx, srcURL)
	if err != nil {
		return err
	}
	defer src.Close()
	dstU, dst, err := openMySQL(ctx, dstURL)
	if err != nil {
		return err
	}
	defer dst.Close()
	_ = srcU
	for _, name := range databases {
		if err := loadDatabase(ctx, src, dst, dstU, name, stdout, stderr); err != nil {
			return err
		}
	}
	return nil
}

func loadDatabase(ctx context.Context, src, dst *sql.DB, dstU *dburl.URL, name string, stdout, stderr io.Writer) error {
	if err := exec(ctx, dst, createDatabase(name)); err != nil {
		return fmt.Errorf("load: create database %s: %w", name, err)
	}
	if err := exec(ctx, dst, useDatabase(name)); err != nil {
		return fmt.Errorf("load: use %s: %w", name, err)
	}
	if err := reportSkipped(ctx, src, name, stderr); err != nil {
		return err
	}
	tables, err := queryStrings(ctx, src, `SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE' ORDER BY table_name`, name)
	if err != nil {
		return fmt.Errorf("load: list tables in %s: %w", name, err)
	}
	refs, err := queryPairs(ctx, src, `SELECT table_name, referenced_table_name FROM information_schema.key_column_usage WHERE table_schema = ? AND referenced_table_name IS NOT NULL`, name)
	if err != nil {
		return fmt.Errorf("load: list foreign keys in %s: %w", name, err)
	}
	tables = orderTables(tables, refs)
	for _, table := range tables {
		if err := loadTable(ctx, src, dst, dstU, name, table, stdout); err != nil {
			return err
		}
	}
	return nil
}

func loadTable(ctx context.Context, src, dst *sql.DB, dstU *dburl.URL, dbName, table string, stdout io.Writer) error {
	create, err := showCreateTable(ctx, src, dbName, table)
	if err != nil {
		return err
	}
	create = RewriteCreateTable(create)
	if _, err := dst.ExecContext(ctx, create); err != nil {
		return fmt.Errorf("load: %s.%s: %w\n%s", dbName, table, err, create)
	}
	qual := qualify(dbName, table)
	probe, err := src.QueryContext(ctx, "SELECT * FROM "+qual+" WHERE 1=0")
	if err != nil {
		return fmt.Errorf("load: %s.%s: %w", dbName, table, err)
	}
	columns, err := probe.Columns()
	probe.Close()
	if err != nil {
		return fmt.Errorf("load: %s.%s: %w", dbName, table, err)
	}
	rows, err := src.QueryContext(ctx, selectAll(dbName, table))
	if err != nil {
		return fmt.Errorf("load: %s.%s: %w", dbName, table, err)
	}
	defer rows.Close()
	n, err := drivers.Copy(ctx, dstU, func() io.Writer { return io.Discard }, func() io.Writer { return io.Discard }, rows, insertTarget(dbName, table, columns))
	if err != nil {
		return fmt.Errorf("load: copy %s.%s: %w", dbName, table, err)
	}
	srcN, err := queryCount(ctx, src, countStar(dbName, table))
	if err != nil {
		return fmt.Errorf("load: count %s.%s on the source: %w", dbName, table, err)
	}
	dstN, err := queryCount(ctx, dst, countStar(dbName, table))
	if err != nil {
		return fmt.Errorf("load: count %s.%s on the destination: %w", dbName, table, err)
	}
	if srcN != dstN {
		return fmt.Errorf("load: %s.%s has %d rows on the source and %d rows on the destination", dbName, table, srcN, dstN)
	}
	_, err = fmt.Fprintf(stdout, "%s.%s %d rows\n", dbName, table, n)
	return err
}

func showCreateTable(ctx context.Context, db *sql.DB, dbName, table string) (string, error) {
	rows, err := db.QueryContext(ctx, showCreate(dbName, table))
	if err != nil {
		return "", fmt.Errorf("load: show create %s.%s: %w", dbName, table, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("load: show create %s.%s returned no row", dbName, table)
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return "", err
	}
	for i, col := range cols {
		if strings.EqualFold(col, "Create Table") {
			stmt := scalarString(vals[i])
			if stmt == "" {
				return "", fmt.Errorf("load: show create %s.%s returned an empty statement", dbName, table)
			}
			return stmt, nil
		}
	}
	return "", fmt.Errorf("load: show create %s.%s has no Create Table column", dbName, table)
}

func reportSkipped(ctx context.Context, db *sql.DB, name string, stderr io.Writer) error {
	views, err := queryStrings(ctx, db, `SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'VIEW' ORDER BY table_name`, name)
	if err != nil {
		return fmt.Errorf("load: list views in %s: %w", name, err)
	}
	triggers, err := queryStrings(ctx, db, `SELECT trigger_name FROM information_schema.triggers WHERE trigger_schema = ? ORDER BY trigger_name`, name)
	if err != nil {
		return fmt.Errorf("load: list triggers in %s: %w", name, err)
	}
	routines, err := queryStrings(ctx, db, `SELECT routine_name FROM information_schema.routines WHERE routine_schema = ? ORDER BY routine_name`, name)
	if err != nil {
		return fmt.Errorf("load: list routines in %s: %w", name, err)
	}
	events, err := queryStrings(ctx, db, `SELECT event_name FROM information_schema.events WHERE event_schema = ? ORDER BY event_name`, name)
	if err != nil {
		return fmt.Errorf("load: list events in %s: %w", name, err)
	}
	for _, view := range views {
		if _, err := fmt.Fprintf(stderr, "skipping view %s.%s\n", name, view); err != nil {
			return err
		}
	}
	for _, trigger := range triggers {
		if _, err := fmt.Fprintf(stderr, "skipping trigger %s.%s\n", name, trigger); err != nil {
			return err
		}
	}
	for _, routine := range routines {
		if _, err := fmt.Fprintf(stderr, "skipping routine %s.%s\n", name, routine); err != nil {
			return err
		}
	}
	for _, event := range events {
		if _, err := fmt.Fprintf(stderr, "skipping event %s.%s\n", name, event); err != nil {
			return err
		}
	}
	return nil
}

// orderTables places referenced tables before the tables that reference them.
// A cycle keeps the remaining names in alphabetical order at the end.
func orderTables(tables []string, refs map[string][]string) []string {
	want := make(map[string]struct{}, len(tables))
	indegree := make(map[string]int, len(tables))
	for _, table := range tables {
		want[table] = struct{}{}
		indegree[table] = 0
	}
	children := map[string][]string{}
	for child, parents := range refs {
		if _, ok := want[child]; !ok {
			continue
		}
		seen := map[string]struct{}{}
		for _, parent := range parents {
			if _, ok := want[parent]; !ok || parent == child {
				continue
			}
			if _, dup := seen[parent]; dup {
				continue
			}
			seen[parent] = struct{}{}
			indegree[child]++
			children[parent] = append(children[parent], child)
		}
	}
	var ready []string
	for _, table := range tables {
		if indegree[table] == 0 {
			ready = append(ready, table)
		}
	}
	sort.Strings(ready)
	var out []string
	for len(ready) > 0 {
		table := ready[0]
		ready = ready[1:]
		out = append(out, table)
		var next []string
		for _, child := range children[table] {
			indegree[child]--
			if indegree[child] == 0 {
				next = append(next, child)
			}
		}
		sort.Strings(next)
		ready = append(ready, next...)
	}
	if len(out) == len(tables) {
		return out
	}
	have := make(map[string]struct{}, len(out))
	for _, table := range out {
		have[table] = struct{}{}
	}
	var left []string
	for _, table := range tables {
		if _, ok := have[table]; !ok {
			left = append(left, table)
		}
	}
	sort.Strings(left)
	return append(out, left...)
}

func exec(ctx context.Context, db *sql.DB, q string) error {
	_, err := db.ExecContext(ctx, q)
	return err
}

func queryStrings(ctx context.Context, db *sql.DB, q, arg string) ([]string, error) {
	rows, err := db.QueryContext(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func queryPairs(ctx context.Context, db *sql.DB, q, arg string) (map[string][]string, error) {
	rows, err := db.QueryContext(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var child, parent string
		if err := rows.Scan(&child, &parent); err != nil {
			return nil, err
		}
		out[child] = append(out[child], parent)
	}
	return out, rows.Err()
}

func queryCount(ctx context.Context, db *sql.DB, q string, args ...any) (int64, error) {
	var n int64
	err := db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}
