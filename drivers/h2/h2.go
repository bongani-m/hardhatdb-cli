// Package h2 defines and registers usql's Apache H2 driver.
//
// See: https://github.com/jmrobles/h2go
// Group: bad
package h2

import (
	"github.com/bongani-m/hardhatdb-cli/drivers"
	_ "github.com/jmrobles/h2go" // DRIVER
)

func init() {
	drivers.Register("h2", drivers.Driver{
		AllowDollar:            true,
		AllowMultilineComments: true,
		AllowCComments:         true,
	})
}
