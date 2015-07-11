package requests

type CaptureRequest struct {
	OrderId              string `json:"order_id,omitempty"`              // This is a Crowdstart-defined internal order ID.  It is stored on Affirm for our reference.
	ShippingCarrier      string `json:"shipping_carrier,omitempty"`      // The shipping carrier used to ship the items in the charge. Ex: USPS
	ShippingConfirmation string `json:"shipping_confirmation,omitempty"` // The shipping confirmation for the shipment.
}
