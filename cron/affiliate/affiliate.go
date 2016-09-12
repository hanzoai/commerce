package affiliate

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/affiliate"
)

func getAffiliates(db *datastore.Datastore) error {
	affs := make([]*affiliate.Affiliate, 0)
	_, err := affiliate.Query(db).GetAll(&affs)
	return err
}
