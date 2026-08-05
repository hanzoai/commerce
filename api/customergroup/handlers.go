package customergroup

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	customergroupModel "github.com/hanzoai/commerce/models/customergroup"
	"github.com/hanzoai/commerce/models/customergroupmembership"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	api := rest.New(customergroupModel.CustomerGroup{})
	api.POST("/:customergroupid/members", namespaced, AddMember)
	api.DELETE("/:customergroupid/members/:userId", namespaced, RemoveMember)
	api.GET("/:customergroupid/members", namespaced, ListMembers)
	api.Route(router, args...)

	rest.New(customergroupmembership.CustomerGroupMembership{}).Route(router, args...)
}

// addMemberRequest represents the body for adding a member.
type addMemberRequest struct {
	UserId string `json:"userId"`
}

// AddMember adds a user to a customer group.
func AddMember(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	groupId := c.Param("customergroupid")

	// Verify group exists
	group := customergroupModel.New(db)
	if err := group.GetById(groupId); err != nil {
		return http.Fail(c, 404, "No customer group found with id: "+groupId, err)
	}

	var req addMemberRequest
	if err := json.DecodeBytes(c.Body(), &req); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	if req.UserId == "" {
		return http.Fail(c, 400, "userId is required", errors.New("missing userId"))
	}

	// Check for duplicate membership
	existing := customergroupmembership.New(db)
	ok, err := existing.Query().
		Filter("CustomerGroupId=", groupId).
		Filter("UserId=", req.UserId).
		Get()
	if err != nil {
		return http.Fail(c, 500, "Failed to query memberships", err)
	}
	if ok {
		return http.Fail(c, 409, "User is already a member of this group", errors.New("duplicate membership"))
	}

	// Create membership
	membership := customergroupmembership.New(db)
	membership.CustomerGroupId = groupId
	membership.UserId = req.UserId

	if err := membership.Create(); err != nil {
		return http.Fail(c, 500, "Failed to create membership", err)
	}

	return http.Render(c, 201, membership)
}

// RemoveMember removes a user from a customer group.
func RemoveMember(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	groupId := c.Param("customergroupid")
	userId := c.Param("userId")

	// Find membership
	membership := customergroupmembership.New(db)
	ok, err := membership.Query().
		Filter("CustomerGroupId=", groupId).
		Filter("UserId=", userId).
		Get()
	if err != nil {
		return http.Fail(c, 500, "Failed to query memberships", err)
	}
	if !ok {
		return http.Fail(c, 404, "Membership not found", errors.New("membership not found"))
	}

	if err := membership.Delete(); err != nil {
		return http.Fail(c, 500, "Failed to delete membership", err)
	}

	c.SetHeader("Content-Type", "application/json")
	return c.Bytes(204, make([]byte, 0))
}

// ListMembers lists all members of a customer group.
func ListMembers(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	groupId := c.Param("customergroupid")

	// Verify group exists
	group := customergroupModel.New(db)
	if err := group.GetById(groupId); err != nil {
		return http.Fail(c, 404, "No customer group found with id: "+groupId, err)
	}

	var members []*customergroupmembership.CustomerGroupMembership
	q := customergroupmembership.Query(db).Filter("CustomerGroupId=", groupId)
	if _, err := q.GetAll(&members); err != nil {
		return http.Fail(c, 500, "Failed to list members", err)
	}

	return http.Render(c, 200, members)
}
