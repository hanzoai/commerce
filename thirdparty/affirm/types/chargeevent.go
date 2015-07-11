package types

type ChargeEvent struct {
	Id            string `json:"id"`                       // The charge event ID
	Created       string `json:"created"`                  // The date and time the event occurred
	TransactionId string `json:"transaction_id,omitempty"` // The transaction ID associated with this event
	Type          string `json:"type"`                     // The charge event type.  One of 'auth', 'capture', 'refund', 'void'

	// Capture & Refund event fields
	Amount int `json:"amount"` // Capture: The amount captured, must be equal to the authorized amount.  Refund: The amount refunded.

	// Capture & Update event fields
	OrderId              string `json:"order_id,omitempty"`              // Both: The order id
	ShippingCarrier      string `json:"shipping_carrier,omitempty"`      // The shipping carrier used to ship the items in the charge.
	ShippingConfirmation string `json:"shipping_confirmation,omitempty"` // The shipping confirmation for the shipment

	// Capture Only
	Fee int `json:"fee"` // The fee associated with the charge

	// Refund Only
	FeeRefunded int `json:"fee_refunded"` // The fee refunded
}
