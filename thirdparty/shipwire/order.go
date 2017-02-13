package shipwire

import (
	"strconv"

	"hanzo.io/models/order"
	"hanzo.io/models/types/fulfillment"
	"hanzo.io/models/user"
	"hanzo.io/util/log"

	. "hanzo.io/thirdparty/shipwire/types"
)

func (c *Client) CreateOrder(ord *order.Order, usr *user.User, serviceLevelCode ServiceLevelCode) (*Response, error) {
	req := OrderRequest{}
	req.CommerceName = "Hanzo"
	req.OrderNo = strconv.Itoa(ord.Number)
	req.ExternalID = ord.Id()
	req.Options.ServiceLevelCode = serviceLevelCode
	req.ShipTo.Name = ord.ShippingAddress.Name
	req.ShipTo.Email = usr.Email
	req.ShipTo.Address1 = ord.ShippingAddress.Line1
	req.ShipTo.Address2 = ord.ShippingAddress.Line2
	req.ShipTo.City = ord.ShippingAddress.City
	req.ShipTo.State = ord.ShippingAddress.State
	req.ShipTo.Country = ord.ShippingAddress.Country
	req.ShipTo.PostalCode = ord.ShippingAddress.PostalCode
	req.Items = make([]Item, len(ord.Items))

	for i, item := range ord.Items {
		req.Items[i] = Item{
			SKU:      item.SKU(),
			Quantity: item.Quantity,
		}
	}

	ctx := ord.Context()
	o := Order{}

	log.Error("Shipwire Req:\n%v", req, ctx)
	res, err := c.Request("POST", "/orders", req, &o)
	if err != nil {
		return res, err
	}

	log.Error("Shipwire Res:\n%v", res, ctx)

	ord.Fulfillment.Type = fulfillment.Shipwire
	ord.Fulfillment.ExternalId = strconv.Itoa(o.ID)

	return res, ord.Update()
}

func (c *Client) GetOrder(ord *order.Order) {
}
