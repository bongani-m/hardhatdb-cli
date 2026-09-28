package migrate

import (
	"fmt"
	"strings"
)

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func qualify(db, table string) string {
	return quoteIdent(db) + "." + quoteIdent(table)
}

// quoteString quotes a MySQL string literal. Backslash escapes stay intact
// when the server has backslash escapes on, which is the default.
func quoteString(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 0:
			b.WriteString(`\0`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\\', '\'':
			b.WriteByte('\\')
			b.WriteByte(s[i])
		default:
			b.WriteByte(s[i])
		}
	}
	b.WriteByte('\'')
	return b.String()
}

func createDatabase(name string) string {
	return "CREATE DATABASE IF NOT EXISTS " + quoteIdent(name)
}

func useDatabase(name string) string {
	return "USE " + quoteIdent(name)
}

func showCreate(db, table string) string {
	return "SHOW CREATE TABLE " + qualify(db, table)
}

func selectAll(db, table string) string {
	return "SELECT * FROM " + qualify(db, table)
}

func countStar(db, table string) string {
	return "SELECT COUNT(*) FROM " + qualify(db, table)
}

func insertTarget(db, table string, columns []string) string {
	quoted := make([]string, len(columns))
	for i, column := range columns {
		quoted[i] = quoteIdent(column)
	}
	return qualify(db, table) + "(" + strings.Join(quoted, ", ") + ")"
}

// FilterWildDo builds CHANGE REPLICATION FILTER for one pattern per database.
func FilterWildDo(databases []string) string {
	parts := make([]string, len(databases))
	for i, db := range databases {
		parts[i] = quoteString(db + ".%")
	}
	return "CHANGE REPLICATION FILTER REPLICATE_WILD_DO_TABLE = (" + strings.Join(parts, ", ") + ")"
}

// ChangeSource builds CHANGE REPLICATION SOURCE TO with auto-position on.
func ChangeSource(host string, port int, user, password string) string {
	return fmt.Sprintf(
		"CHANGE REPLICATION SOURCE TO SOURCE_HOST = %s, SOURCE_PORT = %d, SOURCE_USER = %s, SOURCE_PASSWORD = %s, SOURCE_AUTO_POSITION = 1",
		quoteString(host), port, quoteString(user), quoteString(password),
	)
}
