package transaction

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/transaction/util"
	"hanzo.io/util/json/http"
	"hanzo.io/log"
)

func List(c *context.Context) {
	id := c.Params.ByName("id")
	kind := c.Params.ByName("kind")

	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)

	res, err := util.GetTransactions(ctx, id, kind, !org.Live)

	if err != nil {
		log.Error("transaction/%v/%v error: '%v'", id, kind, err, c)
		http.Fail(c, 400, err.Error(), err)
		return
	}

	http.Render(c, 200, res)
}
