package responses

import (
	"crowdstart.com/thirdparty/affirm/types"
)

type AuthorizeResponse struct {
	Id       string              `json:"id"`        // The ID of the charge
	Amount   int                 `json:"amount"`    // The initial amount of the charge
	Created  string              `json:"created"`   // The date and time the charge was created.
	Currency string              `json:"currency"`  // The currency type of the charge (uses the same abbreviations as our Currency)
	AuthHold int                 `json:"auth_hold"` // The amount that is in auth hold
	Payable  int                 `json:"payable"`   // The amount that is payable to the merchant
	Void     bool                `json:"void"`      // True if the charge has been voided, False otherwise
	Events   []types.ChargeEvent `json:"events"`    // A list of the charge events.
	Details  types.Checkout      `json:"details"`   // The checkout object associated with the charge.
}
