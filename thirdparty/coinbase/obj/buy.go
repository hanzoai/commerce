package obj

type Buy struct {
	Id            string // Resource ID
	Status        string // Status of the buy. Enum, possible values: created, completed, canceled
	Amount        string // Amount in bitcoin, litecoin, or ethereum
	Total         string // Fiat amount with fees
	Subtotal      string // Fiat amount without fees
	Fee           string // Fee associated with this buy
	Created_At    string
	Updated_At    string
	Resource      string // Constant for this object: "buy"
	Resource_Path string
	Committed     bool
	Instant       bool
	Payout_At     string // Time stamp
}
