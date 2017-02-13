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
	case "return.created", "return.updated", "return.canceled", "return.completed":
		var r Return
		if err := json.Unmarshal(req.Body.Resource, &r); err != nil {
			log.Error("Failed decode resource: %v\n%s", err, req.Body.Resource, c)
		} else {
			updateReturn(c, r)
		}
	case "order.hold.added", "order.hold.cleared":
		var rsrc Resource
		if err := json.Unmarshal(req.Body.Resource, &rsrc); err != nil {
			log.Error("Failed decode resource: %v\n%s", err, req.Body.Resource, c)
		} else {
			holds := make([]Hold, 0)
			for i := range rsrc.Items {
				var h Hold
				if err := json.Unmarshal(rsrc.Items[i].Resource, &h); err != nil {
					log.Error("Failed decode hold: %v\n%s", err, rsrc.Items[i].Resource, c)
				} else {
					holds = append(holds, h)
				}
			}

			updateHolds(c, holds)
		}
	case "tracking.created", "tracking.updated", "tracking.delivered":
		var rsrc Resource
		if err := json.Unmarshal(req.Body.Resource, &rsrc); err != nil {
			log.Error("Failed decode resource: %v\n%s", err, req.Body.Resource, c)
		} else {
			trackings := make([]Tracking, 0)
			for i := range rsrc.Items {
				var t Tracking
				if err := json.Unmarshal(rsrc.Items[i].Resource, &t); err != nil {
					log.Error("Failed decode tracking: %v\n%s", err, rsrc.Items[i].Resource, c)
				} else {
					trackings = append(trackings, t)
				}
			}

			updateTrackings(c, trackings, false)
		}
	case "return.tracking.created", "return.tracking.updated", "return.tracking.delivered":
		var rsrc Resource
		if err := json.Unmarshal(req.Body.Resource, &rsrc); err != nil {
			log.Error("Failed decode resource: %v\n%s", err, req.Body.Resource, c)
		} else {
			trackings := make([]Tracking, 0)
			for i := range rsrc.Items {
				var t Tracking
				if err := json.Unmarshal(rsrc.Items[i].Resource, &t); err != nil {
					log.Error("Failed decode tracking: %v\n%s", err, rsrc.Items[i].Resource, c)
				} else {
					trackings = append(trackings, t)
				}
			}

			updateTrackings(c, trackings, true)
		}
		// case "return.hold.added", "return.hold.cleared":
	}

	c.String(200, "ok\n")
}
