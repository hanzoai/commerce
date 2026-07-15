package transaction

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/util/json/http"
)

func List(c *zip.Ctx) error {
	id := c.Param("id")
	kind := c.Param("kind")

	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c.Context())

	res, err := util.GetTransactions(ctx, id, kind, !org.Live)

	if err != nil {
		log.Error("transaction/%v/%v error: '%v'", id, kind, err, c)
		return http.Fail(c, 400, err.Error(), err)
	}

	return http.Render(c, 200, res)
}
