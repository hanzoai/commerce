package types

type Name struct {
	First  string `json:"first,omitempty"`  // First name
	Last   string `json:"last,omitempty"`   // Last name
	Full   string `json:"full,omitempty"`   // Full name
	Middle string `json:"middle,omitempty"` // middle name
}
