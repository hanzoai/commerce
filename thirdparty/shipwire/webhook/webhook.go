package webhook

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/util/json"
	"hanzo.io/util/json/http"

	. "hanzo.io/thirdparty/shipwire/types"
)

// Process individual webhooks
func Process(c *gin.Context) {
	var req Message
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	switch req.Topic {
	case "order.created", "order.updated", "order.canceled", "order.completed":
		var o Order
		if err := json.Unmarshal(req.Body.Resource, &o); err != nil {
			msg := fmt.Sprintf("Failed decode resource: %v\n%v", err, req.Body.Resource)
			http.Fail(c, 400, msg, err)
		}
		updateOrder(c, o)
	case "order.hold.added", "order.hold.cleared":
		var h Hold
		if err := json.Unmarshal(req.Body.Resource, &h); err != nil {
			msg := fmt.Sprintf("Failed decode resource: %v\n%v", err, req.Body.Resource)
			http.Fail(c, 400, msg, err)
		}
		updateHold(c, h)
	case "tracking.created", "tracking.updated", "tracking.delivered":
		var t Tracking
		if err := json.Unmarshal(req.Body.Resource, &t); err != nil {
			msg := fmt.Sprintf("Failed decode resource: %v\n%v", err, req.Body.Resource)
			http.Fail(c, 400, msg, err)
		}
		updateTracking(c, t)
	case "return.created", "return.updated", "return.canceled", "return.completed":
		var r Return
		if err := json.Unmarshal(req.Body.Resource, &r); err != nil {
			msg := fmt.Sprintf("Failed decode resource: %v\n%v", err, req.Body.Resource)
			http.Fail(c, 400, msg, err)
		}
		updateReturn(c, r)
	case "return.hold.added", "return.hold.cleared":
		c.String(200, "ok\n")
	case "return.tracking.created", "return.tracking.updated", "return.tracking.delivered":
		c.String(200, "ok\n")
	default:
		c.String(200, "ok\n")
	}
}
