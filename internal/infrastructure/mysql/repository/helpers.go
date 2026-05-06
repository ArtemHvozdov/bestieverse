package repository

import (
	"database/sql"
	"encoding/json"
)

// nullJSON converts a json.RawMessage to sql.NullString.
// nil or empty JSON becomes a NULL database value.
func nullJSON(data json.RawMessage) sql.NullString {
	if len(data) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(data), Valid: true}
}
