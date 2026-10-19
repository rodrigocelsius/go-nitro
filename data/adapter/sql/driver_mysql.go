//go:build !nitro_nomysql

package sqladapter

import (
	"errors"

	"github.com/go-sql-driver/mysql"
)

// MySQL/MariaDB driver. Exclude with -tags nitro_nomysql.
func init() {
	mysqlRetryable = func(err error) bool {
		var e *mysql.MySQLError
		return errors.As(err, &e) && (e.Number == 1213 || e.Number == 1205)
	}
}
