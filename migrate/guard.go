package migrate

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/xo/dburl"
)

// ReplicateGuard is the state checked before START REPLICA.
type ReplicateGuard struct {
	SourceTLS           bool
	DatabasesWithTables []string
	ExecutedGTID        string
}

// CheckReplicate refuses a source that asks for TLS, a destination that
// already has tables, and a destination that already has an executed GTID set.
// Those last two replay the binlog and insert the same rows again.
func CheckReplicate(g ReplicateGuard) error {
	if g.SourceTLS {
		return errors.New("replicate: source URL enables TLS, and the replica connects with TLS disabled")
	}
	if len(g.DatabasesWithTables) > 0 {
		return fmt.Errorf("replicate: %s already has tables, and starting replication would insert those rows again", strings.Join(g.DatabasesWithTables, ", "))
	}
	if strings.TrimSpace(g.ExecutedGTID) != "" {
		return fmt.Errorf("replicate: destination executed GTID set is %s, and starting replication would insert those rows again", strings.TrimSpace(g.ExecutedGTID))
	}
	return nil
}

// SourceUsesTLS reports whether the URL asks the MySQL driver to use TLS.
// An empty value leaves TLS off. false, 0, and off do the same.
func SourceUsesTLS(q url.Values) bool {
	switch strings.ToLower(strings.TrimSpace(q.Get("tls"))) {
	case "", "false", "0", "off":
		return false
	default:
		return true
	}
}

// ReplicaStatus is the slice of SHOW REPLICA STATUS this command uses.
type ReplicaStatus struct {
	Present       bool
	IORunning     string
	SQLRunning    string
	ExecutedGTID  string
	RetrievedGTID string
	SecondsBehind sql.NullInt64
}

// ReadyToCutOver reports whether the replica has applied every retrieved GTID
// and both threads are running.
func ReadyToCutOver(s ReplicaStatus) bool {
	if !s.Present {
		return false
	}
	if s.IORunning != "Yes" || s.SQLRunning != "Yes" {
		return false
	}
	if s.ExecutedGTID != s.RetrievedGTID {
		return false
	}
	return s.SecondsBehind.Valid && s.SecondsBehind.Int64 == 0
}

func replicationSource(u *dburl.URL) (host string, port int, user, password string, err error) {
	host = u.Hostname()
	if host == "" {
		return "", 0, "", "", errors.New("replicate: source URL has no host")
	}
	port = 3306
	if raw := u.Port(); raw != "" {
		port, err = strconv.Atoi(raw)
		if err != nil || port < 1 {
			return "", 0, "", "", fmt.Errorf("replicate: source port %q is invalid", raw)
		}
	}
	if u.User == nil || u.User.Username() == "" {
		return "", 0, "", "", errors.New("replicate: source URL has no user")
	}
	user = u.User.Username()
	password, _ = u.User.Password()
	return host, port, user, password, nil
}

func isSystemDatabase(name string) bool {
	switch strings.ToLower(name) {
	case "mysql", "information_schema", "performance_schema", "sys":
		return true
	default:
		return false
	}
}
