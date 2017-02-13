package webhook

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/util/log"

	. "hanzo.io/thirdparty/shipwire/types"
)

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

	ord.Fulfillment.Return.CancelledAt = rtn.Events.Resource.CancelledDate.Time
	ord.Fulfillment.Return.CompletedAt = rtn.Events.Resource.CompletedDate.Time
	ord.Fulfillment.Return.UpdatedAt = rtn.LastUpdatedDate.Time
	ord.Fulfillment.Return.ExpectedAt = rtn.ExpectedDate.Time
	ord.Fulfillment.Return.DeliveredAt = rtn.Events.Resource.DeliveredDate.Time
	ord.Fulfillment.Return.PickedUpAt = rtn.Events.Resource.PickedUpDate.Time
	ord.Fulfillment.Return.ProcessedAt = rtn.Events.Resource.ProcessedDate.Time
	ord.Fulfillment.Return.ReturnedAt = rtn.Events.Resource.ReturnedDate.Time
	ord.Fulfillment.Return.SubmittedAt = rtn.Events.Resource.SubmittedDate.Time

	ord.MustPut()

	c.String(200, "ok\n")
}
