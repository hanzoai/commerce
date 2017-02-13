package webhook

import (
	"net/http/httputil"

	"github.com/gin-gonic/gin"

	"hanzo.io/util/json"
	"hanzo.io/util/log"

	. "hanzo.io/thirdparty/shipwire/types"
)

// Process individual webhooks
func Process(c *gin.Context) {
	dump, _ := httputil.DumpRequest(c.Request, true)
	log.Info("Webhook request:\n%s", dump, c)

	var req Message
	if err := json.Decode(c.Request.Body, &req); err != nil {
		log.Error("Failed to decode request body: %v", err, c)
		c.String(200, "ok\n")
		return
	}

	switch req.Topic {
	case "order.created", "order.updated", "order.canceled", "order.completed":
		var o Order
		if err := json.Unmarshal(req.Body.Resource, &o); err != nil {
			log.Error("Failed decode resource: %v\n%s", err, req.Body.Resource, c)
		} else {
			updateOrder(c, o)
		}
	case "order.hold.added", "order.hold.cleared":
		var h Hold
		if err := json.Unmarshal(req.Body.Resource, &h); err != nil {
			log.Error("Failed decode resource: %v\n%s", err, req.Body.Resource, c)
		} else {
			updateHold(c, h)
		}
	case "tracking.created", "tracking.updated", "tracking.delivered":
		var t Tracking
		if err := json.Unmarshal(req.Body.Resource, &t); err != nil {
			log.Error("Failed decode resource: %v\n%s", err, req.Body.Resource, c)
		} else {
			updateTracking(c, t, false)
		}
	case "return.created", "return.updated", "return.canceled", "return.completed":
		var r Return
		if err := json.Unmarshal(req.Body.Resource, &r); err != nil {
			log.Error("Failed decode resource: %v\n%s", err, req.Body.Resource, c)
		} else {
			updateReturn(c, r)
		}
	case "return.tracking.created", "return.tracking.updated", "return.tracking.delivered":
		var t Tracking
		if err := json.Unmarshal(req.Body.Resource, &t); err != nil {
			log.Error("Failed decode resource: %v\n%s", err, req.Body.Resource, c)
		} else {
			updateTracking(c, t, true)
		}
		// case "return.hold.added", "return.hold.cleared":
	}

	c.String(200, "ok\n")
}
