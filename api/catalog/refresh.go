package catalog

import (
	"os"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/catalogentry"
	"github.com/hanzoai/commerce/util/json/http"
)

// RefreshModels PULLS the upstream catalog and lands it. POST /catalog/models is
// the PUSH of already-decided rows; this is the same landing, for an upstream
// that has to be read rather than told. Both funnel through
// catalogentry.UpsertModels, so the rule that a sync owns cost and admin owns
// price is enforced in one place regardless of which door a row came through.
//
// Scheduling is a CronJob curling this with the service token — the same shape
// the contributor payout already uses, so there is one way to run a periodic
// job:
//
//	curl -XPOST -H "Authorization: Bearer $COMMERCE_SERVICE_TOKEN" \
//	  http://commerce.hanzo.svc:8001/v1/catalog/models/refresh
//
// It is deliberately NOT run at boot: a boot-time call to a third party is a
// boot hazard, and the catalog that is already stored is the one to serve until
// a scheduled run says otherwise.
func RefreshModels(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	rows, err := catalogentry.FetchOpenRouter(c.Context(), os.Getenv("OPENROUTER_API_KEY"))
	if err != nil {
		// Nothing has been written, so the last good catalog stands. A sync that
		// cannot see its upstream must never conclude the upstream is empty —
		// that conclusion would withdraw every model we resell.
		return http.Fail(c, 502, "failed to read the upstream model catalog", err)
	}
	res, err := catalogentry.Refresh(catalogDB(c), catalogentry.ServesOpenRouter, rows)
	if err != nil {
		return http.Fail(c, 500, "failed to refresh the model catalog", err)
	}
	return http.Render(c, 200, res)
}
