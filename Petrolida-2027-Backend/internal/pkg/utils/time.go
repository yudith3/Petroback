package utils

import "time"

// OptionalTime mengembalikan nil untuk zero time supaya field JSON-nya bernilai null.
func OptionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}
