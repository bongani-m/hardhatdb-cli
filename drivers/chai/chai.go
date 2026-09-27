// Package chai defines and registers usql's ChaiSQL driver.
//
// See: https://github.com/chaisql/chai
package chai

import (
	"github.com/bongani-m/hardhatdb-cli/drivers"
	_ "github.com/chaisql/chai" // DRIVER
)

func init() {
	drivers.Register("chai", drivers.Driver{})
}
