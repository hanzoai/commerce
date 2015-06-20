package amazon

type Address struct {
	Name          string //max length: 50 chars, required
	AddressLine1  string //max length: 180 chars, required
	AddressLine2  string //max length: 60 chars, optional
	AddressLine3  string //max length: 60 chars, optional
	City          string //max length: 50 chars, required
	County        string //max length: 50 chars, optional
	District      string //max length: 50 chars, optional
	StateOrRegion string //max length: 50 chars, required
	PostalCode    string //max length: 20 chars, required
	CountryCode   string //Country Code in ISO-3166, required
	Phone         string //phone number, required
}
