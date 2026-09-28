package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io"

	"github.com/bongani-m/hardhatdb-cli/drivers"
	"github.com/xo/dburl"
)

func openMySQL(ctx context.Context, raw string) (*dburl.URL, *sql.DB, error) {
	u, err := dburl.Parse(raw)
	if err != nil {
		return nil, nil, err
	}
	if err := requireMySQL(u); err != nil {
		return nil, nil, err
	}
	db, err := drivers.Open(ctx, u, func() io.Writer { return io.Discard }, func() io.Writer { return io.Discard })
	if err != nil {
		return nil, nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, nil, err
	}
	return u, db, nil
}

func requireMySQL(u *dburl.URL) error {
	if u.Driver != "mysql" {
		return fmt.Errorf("%s is not a MySQL protocol URL", u.Scheme)
	}
	return nil
}

// Status prints SHOW REPLICA STATUS.
func Status(ctx context.Context, dstURL string, stdout io.Writer) error {
	_, db, err := openMySQL(ctx, dstURL)
	if err != nil {
		return err
	}
	defer db.Close()
	st, err := ReadReplicaStatus(ctx, db)
	if err != nil {
		return err
	}
	return FormatStatus(stdout, st)
}
