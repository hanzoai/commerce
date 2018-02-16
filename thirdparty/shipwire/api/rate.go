package api

import (
	"fmt"
	"net/http/httputil"

	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/thirdparty/shipwire"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"
)

func rate(c *context.Context) {
	dump, _ := httputil.DumpRequest(c.Request, true)
	log.Info("Rate request:\n%s", dump, c)

	ord := new(order.Order)
	if err := json.Decode(c.Request.Body, ord); err != nil {
		http.Fail(c, 400, fmt.Errorf("Failed to decode request body: %v", err), err)
		return
	}

	org := middleware.GetOrganization(c)
	client := shipwire.New(c, org.Shipwire.Username, org.Shipwire.Password)
	rates, res, err := client.Rate(ord)
	if err == nil {
		http.Render(c, 200, rates)
	} else {
		http.Fail(c, res.Status, fmt.Errorf("Failed to get rates from Shipwire: %v", err), err)
	}
}
