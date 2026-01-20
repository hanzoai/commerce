package marketing

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/marketing"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/marketing/types"
)

func create(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	req := CreateInput{}

	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, &req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if cmpgn, err := marketing.Create(db, req); err != nil {
		http.Fail(c, 400, "Failed to create campaign", err)
		return
	} else {
		http.Render(c, 201, cmpgn)
	}
}
