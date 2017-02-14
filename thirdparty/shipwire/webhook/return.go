package webhook

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	return_ "hanzo.io/models/return"
	"hanzo.io/util/log"

	. "hanzo.io/thirdparty/shipwire/types"
)

func updateReturn(c *gin.Context, r Return) {
	log.Info("Update order information:\n%v", r, c)

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	rtn := return_.New(db)
	id := r.ExternalID
	if err := rtn.GetById(id); err != nil {
		log.Warn("New return detected '%s'", c)
	}

	ord := order.New(db)
	if ok, err := ord.Query().Filter("Fulfillment.ExternalId=", r.ExternalID).Get(); err != nil {
		log.Warn("Unable to find order '%s': %v", r.ExternalID, err, c)
		c.String(200, "ok\n")
		return
	} else if !ok {
		log.Warn("Unable to find order '%s'", r.ExternalID, err, c)
		c.String(200, "ok\n")
		return
	}

	rtn.CancelledAt = r.Events.Resource.CancelledDate.Time
	rtn.CompletedAt = r.Events.Resource.CompletedDate.Time
	rtn.UpdatedAt = r.LastUpdatedDate.Time
	rtn.ExpectedAt = r.ExpectedDate.Time
	rtn.DeliveredAt = r.Events.Resource.DeliveredDate.Time
	rtn.PickedUpAt = r.Events.Resource.PickedUpDate.Time
	rtn.ProcessedAt = r.Events.Resource.ProcessedDate.Time
	rtn.ReturnedAt = r.Events.Resource.ReturnedDate.Time
	rtn.SubmittedAt = r.Events.Resource.SubmittedDate.Time
	rtn.OrderId = ord.Id()
	rtn.UserId = ord.UserId
	rtn.StoreId = ord.StoreId
	rtn.Status = r.Status

	// need to query something like
	// "items": {"resourceLocation": "http://api.shipwire.com/api/v3/returns/673/items?offset=0&limit=20&expand=all"},
	rtn.MustPut()

	c.String(200, "ok\n")
}
