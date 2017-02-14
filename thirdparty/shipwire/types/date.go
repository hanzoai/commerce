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

	if t, err := time.Parse("2006-01-02T15:04:05-07:00", string(data)); err == nil {
		d.Time = t
		return nil
	}

	if t, err := time.Parse("2006-01-02 15:04:05", string(data)); err == nil {
		d.Time = t
		return nil
	}

	return d.Time.UnmarshalJSON(data)
}
