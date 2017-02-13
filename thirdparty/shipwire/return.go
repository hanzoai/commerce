package shipwire

import (
	"strconv"

	"hanzo.io/models/order"
	. "hanzo.io/thirdparty/shipwire/types"
)

func (c *Client) CreateReturn(ord *order.Order) (*Response, error) {
	req := ReturnRequest{}
	req.ExternalID = ord.Id()

	id, err := strconv.Atoi(ord.Fulfillment.ExternalId)
	if err != nil {
		return nil, err
	}

	req.OriginalOrder.ID = id
	req.Options.EmailCustomer = 1
	req.Options.GeneratePrepaidLabel = 1

	for i, item := range ord.Items {
		req.Items[i] = Item{
			SKU:      item.SKU(),
			Quantity: item.Quantity,
		}
	}

	return c.Request("POST", "/returns", req, nil)
}
