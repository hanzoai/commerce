package customergroupmembership

import (
	"github.com/hanzoai/commerce/models/mixin"
)

type CustomerGroupMembership struct {
	mixin.Model

	CustomerGroupId string `json:"customerGroupId"`
	UserId          string `json:"userId"`
}
