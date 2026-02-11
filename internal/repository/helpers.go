package repository

import (
	"database/sql"
	"errors"
	"time"
)

// ErrNotFound is returned when a queried entity does not exist.
var ErrNotFound = errors.New("not found")

// dateLayout is the standard date format for project/node/work-item dates in SQLite.
const dateLayout = "2006-01-02"

// parseNullableTime parses a sql.NullString into a *time.Time using the given layout.
// Returns nil if the value is NULL, empty, or fails to parse.
func parseNullableTime(s sql.NullString, layout string) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t, err := time.Parse(layout, s.String)
	if err != nil {
		return nil
	}
	return &t
}

// nullableTimeToString converts a *time.Time to a value suitable for SQLite storage.
// Returns nil (SQL NULL) if the pointer is nil, otherwise returns the formatted string.
func nullableTimeToString(t *time.Time, layout string) interface{} {
	if t == nil {
		return nil
	}
	return t.Format(layout)
}

// nullableIntToValue converts a *int to a value suitable for SQLite storage.
// Returns nil (SQL NULL) if the pointer is nil, otherwise returns the int value.
func nullableIntToValue(v *int) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

// boolToInt converts a Go bool to an integer (0 or 1) for SQLite storage.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// intToBool converts a SQLite integer (0 or 1) to a Go bool.
func intToBool(i int) bool {
	return i != 0
}

// nowUTC returns the current UTC time formatted as RFC3339.
func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
