package api

import (
	"fmt"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/thirdparty/shipwire"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

func rate(c *zip.Ctx) error {
	log.Info("Rate request:\n%s", c.Fiber().Request().String(), c)

	ord := new(order.Order)
	if err := json.Unmarshal(c.Body(), ord); err != nil {
		return http.Fail(c, 400, fmt.Errorf("Failed to decode request body: %v", err), err)
	}

	org := middleware.GetOrganization(c)
	client := shipwire.New(c, org.Shipwire.Username, org.Shipwire.Password)
	rates, res, err := client.Rate(ord)
	if err == nil {
		return http.Render(c, 200, rates)
	}
	return http.Fail(c, res.Status, fmt.Errorf("Failed to get rates from Shipwire: %v", err), err)
}
