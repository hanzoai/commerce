package events

// Standard commerce event subjects for NATS/JetStream.
const (
	SubjectOrderCreated    = "commerce.order.created"
	SubjectOrderCompleted  = "commerce.order.completed"
	SubjectOrderCanceled   = "commerce.order.canceled"
	SubjectOrderRefunded   = "commerce.order.refunded"
	SubjectCheckoutStarted = "commerce.checkout.started"
	SubjectCheckoutFailed  = "commerce.checkout.failed"
	SubjectPaymentReceived = "commerce.payment.received"
	SubjectCartUpdated     = "commerce.cart.updated"
	SubjectProductViewed   = "commerce.product.viewed"
)

// StreamName is the JetStream stream for commerce events.
const StreamName = "COMMERCE"

// StreamSubjects defines what subjects the COMMERCE stream captures.
var StreamSubjects = []string{"commerce.>"}
