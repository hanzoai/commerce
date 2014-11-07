package models

type Category int

const (
	Art Category = iota
	Technology
	Music
)

type Campaign struct {
	Id              string
	Approved		bool
	Enabled			bool
	Category        Category
	Title           string
	Tagline         string
	PitchMedia      string
	VideoUrl        string
	VideoOverlayUrl string
	ImageUrl        string
	Description     string
	StripeKey       string
	Backers         int
	Raised          int64
	Thumbnail       string
	OriginalUrl     string
	StoreUrl        string
	Products        []Product
	Members         []User
	Creator			User
	Fundee          struct {
		BusinessName        string
		DBA                 string
		Website             string
		TaxId               string
		Country             string
		Owner               string
		PrimaryContact      string
		PrimaryContactPhone string
	}
	PayPalConnected bool
	PayPalApiKeys   string
	StripeToken     string `schema:"-"`
	GoogleAnalytics string
	FacebookTag     string
	Links           []string
}
