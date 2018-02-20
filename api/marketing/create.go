package marketing

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/marketing"
	"hanzo.io/middleware"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"

	. "hanzo.io/marketing/types"
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
