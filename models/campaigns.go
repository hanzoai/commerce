package models

type Category int

const (
	Art Category = iota
	Technology
	Music
)

type SalesforceTokens struct {
	AccessToken  string
	RefreshToken string
	InstanceUrl  string
	Id           string
	IssuedAt     string
	Signature    string
}

type Campaign struct {
	Id              string
	Approved        bool
	Enabled         bool
	Category        Category
	Title           string
	Tagline         string
	PitchMedia      string
	VideoUrl        string
	VideoOverlayUrl string
	ImageUrl        string
	Description     string
	Backers         int
	Raised          int64
	Thumbnail       string
	OriginalUrl     string
	StoreUrl        string
	Products        []Product `datastore:"-"`
	Members         []User    `datastore:"-"`
	Creator         User      `datastore:"-"`
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
	Salesforce      SalesforceTokens
	Stripe          struct {
		AccessToken    string
		Livemode       bool
		PublishableKey string
		RefreshToken   string
		Scope          string
		TokenType      string
		UserId         string
	}
	GoogleAnalytics string
	FacebookTag     string
	Links           []string
}
