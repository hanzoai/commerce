package types

import "time"

type Date struct {
	time.Time
}

// Patch for processing null values
func (d *Date) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if string(data) == "null" {
		return nil
	}

	return d.Time.UnmarshalJSON(data)
}
