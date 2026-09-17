package http

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/util/hashid"
	"github.com/hanzoai/commerce/util/template"
)

func decodeKey(c *zip.Ctx) error {
	ctx := c.Context()
	id := c.Param("id")
	key, err := hashid.DecodeKey(ctx, id)
	if err != nil {
		panic(err)
	}
	return template.Render(c, "hashid.html",
		"id", id,
		"namespace", key.Namespace(),
		"kind", key.Kind(),
		"parent", key.Parent(),
		"intid", key.IntID(),
	)
}

// Setup handlers for HTTP registered tasks
func SetupRoutes(router *zip.Group) {
	// Redirects
	router.Raw(http.MethodGet, "/hashid", func(c *zip.Ctx) error {
		return template.Render(c, "hashid.html")
	})

	// Check a hashid
	router.Raw(http.MethodGet, "/hashid/:id", decodeKey)
	router.Raw(http.MethodPost, "/hashid/:id", decodeKey)
}
