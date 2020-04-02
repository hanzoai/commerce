package checkout

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/api/checkout/stripe"
	"hanzo.io/log"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/product"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/counter"
	"hanzo.io/util/json"
)

func refund(c *gin.Context, org *organization.Organization, ord *order.Order) error {
	type Refund struct {
		Amount currency.Cents `json:"amount"`
	}

	req := new(Refund)
	if err := json.Decode(c.Request.Body, req); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Request.Body, err, c)
		return FailedToDecodeRequestBody
	}

	log.JSON(req)

	if err := stripe.Refund(org, ord, req.Amount); err != nil {
		return err
	}

	if !ord.Test {
		if err := counter.IncrOrderRefund(ord.Context(), ord, int(req.Amount), time.Now()); err != nil {
			log.Error("IncrOrderRefund Error %v", err, c)
		}

		if ord.Total == ord.Refunded {
			if err := ord.GetItemEntities(); err != nil {
				for _, item := range ord.Items {
					prod := product.New(ord.Db)

					if err := prod.GetById(item.ProductId); err != nil {
						log.Error("no product found %v", err, c)
					}

					counter.IncrProductRefund(ord.Context(), prod, ord)
				}
			}
		}
	}

	return nil
}
