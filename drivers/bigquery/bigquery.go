// Package bigquery defines and registers usql's Google BigQuery driver.
//
// See: https://github.com/go-gorm/bigquery
package bigquery

import (
	"github.com/bongani-m/hardhatdb-cli/drivers"
	_ "gorm.io/driver/bigquery/driver" // DRIVER
)

func init() {
	drivers.Register("bigquery", drivers.Driver{})
}
