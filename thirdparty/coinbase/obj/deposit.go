package obj

type Deposit struct {
	Id            string    // Resource ID
	Status        string    // Status of the deposit. Enum, possible values: created, completed, canceled
	Amount        MoneyHash // Amount
	Subtotal      MoneyHash // Amount without fees
	Fee           MoneyHash // Fees associated to this deposit
	Created_At    string    // Timestamp of creation
	Updated_At    string    // Timestamp of update
	Resource      string    // Constant: deposit
	Resource_Path string
	Committed     bool // Has this deposit being committed
	Payout_At     string
}
