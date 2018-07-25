package subscription

import (
	"strings"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/deprecated/subscription"
	"hanzo.io/models/user"
)

type SubscriptionReq struct {
	User_         *user.User                 `json:"user"`
	Subscription_ *subscription.Subscription `json:"subscription"`
	Db            *datastore.Datastore
}

func (sr *SubscriptionReq) User() (*user.User, error) {
	db := sr.Db
	// Pull user id off request
	id := sr.User_.Id_

	// If id is set, this is a pre-existing user, use data from datastore
	if id != "" {
		sr.User_ = user.New(db)
		if err := sr.User_.GetById(id); err != nil {
			return nil, UserDoesNotExist
		} else {
			return sr.User_, nil
		}
	}

	// Ensure model mixin is setup correctly
	sr.User_.Model = mixin.Model{Db: db, Entity: sr.User_}

	// Normalize a few things we get in
	sr.User_.Email = strings.ToLower(strings.TrimSpace(sr.User_.Email))
	sr.User_.Username = strings.ToLower(strings.TrimSpace(sr.User_.Username))

	return sr.User_, nil
}

func (sr *SubscriptionReq) Subscription() (*subscription.Subscription, error) {
	sub := sr.Subscription_
	sub.Model.Entity = sr.Subscription_
	sub.Model.Db = sr.Db

	return sub, nil
}
