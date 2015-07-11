package types

type Address struct {
	Line1   string `json:"line1"`           // Street address
	Line2   string `json:"line2,omitempty"` // Second line of address
	City    string `json:"city"`            // City
	State   string `json:"state"`           // State
	Zipcode string `json:"zipcode"`         // Zipcode
	Country string `json:"country"`         // Country code
}
