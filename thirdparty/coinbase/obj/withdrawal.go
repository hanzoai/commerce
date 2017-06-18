package obj

type Withdrawal struct {
	Id            string    // Resource ID
	Status        string    // Status of the withdrawal. Enum, values: created, completed, canceled
	Amount        MoneyHash // Amount
	Subtotal      MoneyHash // Amount without fees
	Fee           MoneyHash // Fee associated with this withdrawal
	Created_At    string
	Updated_At    string
	Resource      string // Constant: Withdrawal
	Resource_Path string
	Committed     bool   // Has this withdrawal been committed
	Payout_At     string // When a withdrawal isn't executed instantly, it will execute at this date.
}
