package balance

import (
	"errors"

	aeds "appengine/datastore"

	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
)

var FailedToCaptureCharge = errors.New("Failed to capture charge")

func Capture(org *organization.Organization, ord *order.Order) (*order.Order, []*aeds.Key, []*payment.Payment, error) {
	return nil, nil, nil, nil
}
