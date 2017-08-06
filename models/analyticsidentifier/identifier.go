package analyticsidentifier

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Ids struct {
	// Unique Identifier set on the client to identify the browser + device
	UUId string `json:"uuid"`

	// These are secondary identifiers that are populated on newsletter signup
	// or user signup/login

	// User Id
	UserId string `json:"userId"`
	// UserIdAddedOn time.Time `json:"userIdAddedOn"`

	// Unused as of yet
	// // Subscriber Id
	// SubId        string    `json:"subscriberId"`
	// SubIdAddedOn time.Time `json:"subIdAddedOn"`

	// These are optional identifiers for third party analytics

	// GA Ids

	// Google Analytics Long Id
	GAId string `json:"ga"`
	// GAIdAddedOn time.Time `json:"gaIdAddedOn"`

	// Google Analytics Short Id
	// GAShortId string `json:"gid"`
	// GAShortIdAddedOn time.Time `json:"gaIdAddedOn"`

	// FB Ids

	// Facebook Id
	FBId string `json:"fr"`
	// FBIdAddedOn time.Time `json:"fbIdAddedOn"`
}

type AnalyticsIdentifier struct {
	mixin.Model
	Ids
}
