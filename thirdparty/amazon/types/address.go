package types

type Address struct {
	Name          string // Max length: 50 chars. Required.
	AddressLine1  string // Max length: 180 chars. Required.
	AddressLine2  string // Max length: 60 chars. Optional.
	AddressLine3  string // Max length: 60 chars. Optional.
	City          string // Max length: 50 chars. Required.
	County        string // Max length: 50 chars. Optional.
	District      string // Max length: 50 chars. Optional.
	StateOrRegion string // Max length: 50 chars. Required.
	PostalCode    string // Max length: 20 chars. Required.
	CountryCode   string // Country Code in ISO-3166. Required.
	Phone         string // Phone number. Required.
}
