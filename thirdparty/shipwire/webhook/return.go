package webhook

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/models/types/fulfillment"
	"hanzo.io/util/log"

	. "hanzo.io/thirdparty/shipwire/types"
)

func convertReturn(rtn Return) fulfillment.Return {
	var r fulfillment.Return
	r.CancelledAt = rtn.Events.Resource.CancelledDate.Time
	r.CompletedAt = rtn.Events.Resource.CompletedDate.Time
	r.UpdatedAt = rtn.LastUpdatedDate.Time
	r.ExpectedAt = rtn.ExpectedDate.Time
	r.DeliveredAt = rtn.Events.Resource.DeliveredDate.Time
	r.PickedUpAt = rtn.Events.Resource.PickedUpDate.Time
	r.ProcessedAt = rtn.Events.Resource.ProcessedDate.Time
	r.ReturnedAt = rtn.Events.Resource.ReturnedDate.Time
	r.SubmittedAt = rtn.Events.Resource.SubmittedDate.Time
	return r
}

func updateReturn(c *gin.Context, rtn Return) {
	log.Warn("Return Information:\n%v", rtn, c)

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	ord := order.New(db)
	id := rtn.ExternalID
	err := ord.GetById(id)
	if err != nil {
		log.Warn("Unable to find order '%s': %v", id, err, c)
		c.String(200, "ok\n")
		return
	}

	// ord.Fulfillment.Returns = []fulfillment.Return{convertReturn(rtn)}

	// ord.MustPut()

	c.String(200, "ok\n")
}
