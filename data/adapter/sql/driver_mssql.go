//go:build !nitro_nomssql

package sqladapter

// SQL Server driver. Exclude with -tags nitro_nomssql.
import _ "github.com/microsoft/go-mssqldb"
