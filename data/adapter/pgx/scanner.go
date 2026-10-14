package pgx

import (
	"github.com/jackc/pgx/v5"
)

// ScanRows scans multiple rows into []map[string]any.
func ScanRows(rows pgx.Rows) ([]map[string]any, error) {
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columns[i] = fd.Name
	}

	var results []map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}

		row := make(map[string]any, len(values))
		for i, fd := range fieldDescriptions {
			row[fd.Name] = values[i]
		}
		results = append(results, row)
	}

	return results, rows.Err()
}
