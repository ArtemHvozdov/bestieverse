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

// scanNullJSON scans a nullable JSON column into json.RawMessage.
// A NULL column value results in a nil RawMessage.
func scanNullJSON(ns sql.NullString) json.RawMessage {
	if !ns.Valid {
		return nil
	}
	return json.RawMessage(ns.String)
}
