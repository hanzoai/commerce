package tasks

import (
	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
)

var AddEmailToContribution = parallel.Task("add-email-to-contribution", func(db *datastore.Datastore, key datastore.Key, contribution models.Contribution) {
	user := new(models.User)
	db.Get(contribution.UserId, user)
	contribution.Email = user.Email
	db.PutKind("contribution", key, &contribution)
})
