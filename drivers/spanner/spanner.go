// Package spanner defines and registers usql's Google Spanner driver.
//
// See: https://github.com/googleapis/go-sql-spanner
package spanner

import (
	"github.com/bongani-m/hardhatdb-cli/drivers"
	_ "github.com/googleapis/go-sql-spanner" // DRIVER
)

func init() {
	drivers.Register("spanner", drivers.Driver{})
}
