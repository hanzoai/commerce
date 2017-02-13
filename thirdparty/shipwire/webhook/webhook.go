package webhook

import (
	json_ "encoding/json"
	"net/http/httputil"

	"github.com/gin-gonic/gin"

	"hanzo.io/util/json"
	"hanzo.io/util/log"

	. "hanzo.io/thirdparty/shipwire/types"
)

func getList(c *gin.Context, data []byte, dst interface{}) error {
	var rsrc Resource
	if err := json.Unmarshal(data, &rsrc); err != nil {
		log.Error("Failed decode resource: %v\n%s", err, data, c)
		return err
	} else {
		resources := make([]json_.RawMessage, 0)
		for i := range rsrc.Items {
			resources = append(resources, rsrc.Items[i].Resource)
		}
		data := json.EncodeBytes(resources)
		if err := json.Unmarshal(data, dst); err != nil {
			log.Error("Failed decode: %v\n%s", err, resources, c)
			return err
		}
	}
	return nil
}

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
	// Single item response
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

	// List of items in response
	case "order.hold.added", "order.hold.cleared":
		holds := make([]Hold, 0)
		if err := getList(c, req.Body.Resource, holds); err != nil {
			updateHolds(c, holds)
		}
	case "tracking.created", "tracking.updated", "tracking.delivered":
		trackings := make([]Tracking, 0)
		if err := getList(c, req.Body.Resource, trackings); err != nil {
			updateTrackings(c, trackings)
		}

	case "return.hold.added", "return.hold.cleared":
		// holds := make([]Hold, 0)
		// if err := getList(c, req.Body.Resource, holds); err != nil {
		// 	updateReturnHolds(c, holds)
		// }
	case "return.tracking.created", "return.tracking.updated", "return.tracking.delivered":
		// trackings := make([]Tracking, 0)
		// if err := getList(c, req.Body.Resource, trackings); err != nil {
		// 	updateReturnTrackings(c, trackings)
		// }
	}

	c.String(200, "ok\n")
}
