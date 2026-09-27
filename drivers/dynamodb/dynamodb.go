// Package dynamodb defines and registers usql's DynamoDb driver.
//
// See: https://github.com/btnguyen2k/godynamo
package dynamodb

import (
	"github.com/bongani-m/hardhatdb-cli/drivers"
	_ "github.com/btnguyen2k/godynamo" // DRIVER
)

func init() {
	drivers.Register("godynamo", drivers.Driver{})
}
