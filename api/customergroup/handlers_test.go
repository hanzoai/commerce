package customergroup

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	customergroupModel "github.com/hanzoai/commerce/models/customergroup"
	"github.com/hanzoai/commerce/models/customergroupmembership"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// customergroupAPI wires the real member add/remove/list handlers behind a seed
// middleware binding org `ns`, so they run their real org.Namespaced datastore path.
type customergroupAPI struct {
	app *zip.App
	db  *datastore.Datastore
}

func newCustomerGroupAPI(ns string) *customergroupAPI {
	base := nscontext.WithNamespace(context.Background(), ns)
	db := datastore.New(base)
	org := organization.New(db)
	org.Name = ns

	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		c.SetContext(base)
		c.Locals("organization", org)
		return c.Next()
	}
	app.Post("/:customergroupid/members", seed, AddMember)
	app.Delete("/:customergroupid/members/:userId", seed, RemoveMember)
	app.Get("/:customergroupid/members", seed, ListMembers)

	return &customergroupAPI{app: app, db: db}
}

func (a *customergroupAPI) do(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		r = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func (a *customergroupAPI) seedGroup(t *testing.T) *customergroupModel.CustomerGroup {
	t.Helper()
	g := customergroupModel.New(a.db)
	g.Name = "VIPs"
	if err := g.Create(); err != nil {
		t.Fatalf("create group: %v", err)
	}
	return g
}

func (a *customergroupAPI) memberCount(t *testing.T, groupId string) int {
	t.Helper()
	var members []*customergroupmembership.CustomerGroupMembership
	if _, err := customergroupmembership.Query(a.db).Filter("CustomerGroupId=", groupId).GetAll(&members); err != nil {
		t.Fatalf("count members: %v", err)
	}
	return len(members)
}

// TestMember_AddListRemove drives the membership lifecycle: add a user, list
// members, remove the user.
func TestMember_AddListRemove(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newCustomerGroupAPI("acme")
	g := api.seedGroup(t)

	// ADD
	if code, body := api.do(t, http.MethodPost, "/"+g.Id()+"/members", map[string]any{"userId": "user-1"}); code != http.StatusCreated {
		t.Fatalf("add member = %d, body=%s", code, body)
	}
	if n := api.memberCount(t, g.Id()); n != 1 {
		t.Fatalf("member count = %d, want 1", n)
	}

	// LIST
	code, body := api.do(t, http.MethodGet, "/"+g.Id()+"/members", nil)
	if code != http.StatusOK {
		t.Fatalf("list = %d, body=%s", code, body)
	}
	var members []*customergroupmembership.CustomerGroupMembership
	if err := json.Unmarshal(body, &members); err != nil {
		t.Fatalf("decode list: %v (%s)", err, body)
	}
	if len(members) != 1 || members[0].UserId != "user-1" {
		t.Fatalf("list = %+v, want [user-1]", members)
	}

	// REMOVE (204 No Content)
	if code, _ := api.do(t, http.MethodDelete, "/"+g.Id()+"/members/user-1", nil); code != http.StatusNoContent {
		t.Fatalf("remove = %d, want 204", code)
	}
	if n := api.memberCount(t, g.Id()); n != 0 {
		t.Fatalf("member count after remove = %d, want 0", n)
	}
}

// TestAddMember_Duplicate: adding an already-present member is a 409 and does
// not create a second membership row.
func TestAddMember_Duplicate(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newCustomerGroupAPI("acme")
	g := api.seedGroup(t)

	if code, _ := api.do(t, http.MethodPost, "/"+g.Id()+"/members", map[string]any{"userId": "user-1"}); code != http.StatusCreated {
		t.Fatalf("first add failed")
	}
	if code, _ := api.do(t, http.MethodPost, "/"+g.Id()+"/members", map[string]any{"userId": "user-1"}); code != http.StatusConflict {
		t.Fatalf("duplicate add = %d, want 409", code)
	}
	if n := api.memberCount(t, g.Id()); n != 1 {
		t.Fatalf("member count = %d, want 1 (no duplicate)", n)
	}
}

// TestAddMember_MissingUserId: a body without userId is a 400.
func TestAddMember_MissingUserId(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newCustomerGroupAPI("acme")
	g := api.seedGroup(t)

	if code, _ := api.do(t, http.MethodPost, "/"+g.Id()+"/members", map[string]any{}); code != http.StatusBadRequest {
		t.Fatalf("missing userId = %d, want 400", code)
	}
}

// TestRemoveMember_NotPresent: removing a nonexistent membership is a 404.
func TestRemoveMember_NotPresent(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newCustomerGroupAPI("acme")
	g := api.seedGroup(t)

	if code, _ := api.do(t, http.MethodDelete, "/"+g.Id()+"/members/ghost", nil); code != http.StatusNotFound {
		t.Fatalf("remove absent member = %d, want 404", code)
	}
}

// TestAddMember_UnknownGroup: adding to a nonexistent group is a 404.
func TestAddMember_UnknownGroup(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newCustomerGroupAPI("acme")
	if code, _ := api.do(t, http.MethodPost, "/nope/members", map[string]any{"userId": "user-1"}); code != http.StatusNotFound {
		t.Fatalf("add to unknown group = %d, want 404", code)
	}
}

// TestCustomerGroup_CrossOrgIsolation: a group in org "acme" is invisible to org
// "other" — memberships are namespace-scoped per tenant.
func TestCustomerGroup_CrossOrgIsolation(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	acme := newCustomerGroupAPI("acme")
	g := acme.seedGroup(t)

	other := newCustomerGroupAPI("other")
	if code, _ := other.do(t, http.MethodPost, "/"+g.Id()+"/members", map[string]any{"userId": "user-1"}); code != http.StatusNotFound {
		t.Fatalf("cross-org add member = %d, want 404 (group not visible)", code)
	}
	if code, _ := other.do(t, http.MethodGet, "/"+g.Id()+"/members", nil); code != http.StatusNotFound {
		t.Fatalf("cross-org list = %d, want 404", code)
	}
}
