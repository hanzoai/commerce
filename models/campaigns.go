package models

type Category int

const (
	Art Category = iota
	Technology
	Music
)

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
	Salesforce      struct {
		AccessToken string
		IssuedAt    string
	}
	Stripe struct {
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

	// TODO: Deprecated, please remove eventually
	StripeToken string
	StripeKey   string
}

type Perk struct {
	Id                string
	Description       string
	EstimatedDelivery string
	GearQuantity      int
	HelmetQuantity    int
	Price             string
	Title             string
}

type Contribution struct {
	Id            string
	Email         string
	FundingDate   string
	PaymentMethod string
	Perk          Perk
	Status        string
}
