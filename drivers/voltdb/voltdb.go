// Package voltdb defines and registers usql's VoltDB driver.
//
// See: https://github.com/VoltDB/voltdb-client-go
package voltdb

import (
	_ "github.com/VoltDB/voltdb-client-go/voltdbclient" // DRIVER
	"github.com/bongani-m/hardhatdb-cli/drivers"
)

func init() {
	drivers.Register("voltdb", drivers.Driver{
		AllowMultilineComments: true,
	})
}
