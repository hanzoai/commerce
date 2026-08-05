package account

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/json/http"
)

func exists(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	emailorusername := c.Param("emailorusername")

	usr := user.New(db)

	if err := usr.GetByEmail(emailorusername); err == nil {
		return http.Render(c, 200, map[string]any{"exists": true})
	} else if err := usr.GetByUsername(emailorusername); err == nil {
		return http.Render(c, 200, map[string]any{"exists": true})
	}
	return http.Render(c, 200, map[string]any{"exists": false})
}
