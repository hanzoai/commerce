package types

import "time"

type Date struct {
	time.Time
}

// Patch for processing null values
func (d *Date) UnmarshalJSON(data []byte) error {
	str := string(data)

	// Ignore null dates
	if str == "null" {
		return nil
	}

	// Shipwire date style 1
	if t, err := time.Parse("2006-01-02T15:04:05-07:00", str); err == nil {
		d.Time = t
		return nil
	}

	// Shipwire date style 2
	if t, err := time.Parse("2006-01-02 15:04:05", str); err == nil {
		d.Time = t
		return nil
	}

	// Fallback to normal Unmarshal method
	return d.Time.UnmarshalJSON(data)
}
