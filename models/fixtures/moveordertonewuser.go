package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"
)

var MoveOrderToNewUser = New("move-order-to-new-user", func(c *gin.Context) {
	oldEmail := "marktwellsa@mac.com"
	newEmail := "marktwells@mac.com"

	db := datastore.New(c)

	org := organization.New(db)
	org.MustGetById("stoned")

	ctx := org.Context()

	log.Warn("Moving '%v' orders to '%v'", oldEmail, newEmail, ctx)
	nsDb := datastore.New(org.Namespaced(ctx))
	oldUsr := user.New(nsDb)
	if err := oldUsr.GetById(oldEmail); err != nil {
		log.Error(err, ctx)
	}

	newUsr := user.New(nsDb)
	if err := newUsr.GetById(newEmail); err != nil {
		log.Error(err, ctx)
	}

	ords := make([]*order.Order, 0)
	if _, err := order.Query(db).
		Filter("UserId=", oldUsr.Id()).
		GetAll(&ords); err != nil {
		log.Error(err, ctx)
	}
	log.Warn("Moving %v orders", len(ords), ctx)

	for _, ord := range ords {
		oldOK := ord.Key()
		newOK := db.NewKey(ord.Kind(), oldOK.StringID(), oldOK.IntID(), newUsr.Key())

		ord.SetKey(newOK)
		if err := ord.Put(); err != nil {
			log.Error(err, ctx)
		}

		ord.SetKey(oldOK)
		if err := ord.Delete(); err != nil {
			log.Error(err, ctx)
		}

		pays := make([]*payment.Payment, 0)
		if _, err := payment.Query(db).
			Filter("OrderId", ord.Id()).
			GetAll(&pays); err != nil {
			log.Error(err, ctx)
		}
		log.Warn("Moving %v payments", len(ords), ctx)

		for _, pay := range pays {
			oldPK := pay.Key()
			newPK := db.NewKey(pay.Kind(), oldPK.StringID(), oldPK.IntID(), newOK)

			pay.SetKey(newPK)
			if err := pay.Put(); err != nil {
				log.Error(err, ctx)
			}

			pay.SetKey(oldPK)
			if err := pay.Delete(); err != nil {
				log.Error(err, ctx)
			}
		}
	}
})
