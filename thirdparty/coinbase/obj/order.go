package obj

type Order struct {
	Id              string    // resource Id
	Code            string    // client facing order code
	Status          string    // Status of the order. Enum, possible values: Active, paid, expired, mispaid.
	Type            string    // Type of the order.  Currently available values: Order, Donation, invoice.
	Name            string    // The name of the item for which you are collecting Bitcoin. For example, Acme Order 123 or Annual Pledge Drive.
	Description     string    // Longer description of the item in case you want it added to the user's transaction notes.
	Amount          MoneyHash // order in original currency (can be fiat or BTC)
	Payment_Amount  MoneyHash // Total payment of the payout that was scheduled to be your bank account using instant payout.
	Bitcoin_Address string    // Bitcoin address for the payment.
	Bitcoin_Amount  MoneyHash // Exchange bitcoin amount for the order.
	Bitcoin_Uri     string    // Uri to open payment in native applications.
	Receipt_Url     string    // Url to order details
	Expires_At      string    // Time of expiration
	Mispaid_At      string    // Time of mispayment
}
