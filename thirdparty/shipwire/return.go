package shipwire

import (
	"errors"
	"strconv"

	"hanzo.io/models/order"
	"hanzo.io/models/types/fulfillment"

	. "hanzo.io/thirdparty/shipwire/types"
)

func (c *Client) CreateReturn(ord *order.Order) (*Response, error) {
	req := ReturnRequest{}
	req.ExternalID = "e" + ord.Id()

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

	rtn := Return{}
	res, err := c.Request("POST", "/returns", req, &rtn)
	if err != nil {
		return res, err
	}

	if res.Status > 299 {
		return res, errors.New("Failed to create return")
	}

	ord.Fulfillment.Status = fulfillment.Returned
	ord.Fulfillment.Return.Status = rtn.Status
	ord.Fulfillment.Return.ExternalId = strconv.Itoa(rtn.ID)
	ord.Fulfillment.Return.ExpectedAt = rtn.ExpectedDate.Time
	ord.Fulfillment.Return.UpdatedAt = rtn.LastUpdatedDate.Time

	return res, ord.Update()
}
