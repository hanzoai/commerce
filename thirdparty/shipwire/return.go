package shipwire

import (
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
	res, err := c.Resource("POST", "/returns", req, &rtn)
	if err != nil {
		return res, err
	}

	ord.Fulfillment.Status = fulfillment.Returned
	// ord.Fulfillment.Returns[0].Status = rtn.Status
	// ord.Fulfillment.Returns[0].ExternalId = strconv.Itoa(rtn.ID)
	// ord.Fulfillment.Returns[0].ExpectedAt = rtn.ExpectedDate.Time
	// ord.Fulfillment.Returns[0].UpdatedAt = rtn.LastUpdatedDate.Time

	return res, ord.Update()
}
