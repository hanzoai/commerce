package site

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/site"
	"hanzo.io/thirdparty/netlify"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
)

func createDeploy(c *context.Context) {
	ctx := middleware.GetAppEngine(c)
	org := middleware.GetOrganization(c)
	siteid := c.Params.ByName("siteid")

	// Get associated site
	db := datastore.New(ctx)
	ste := site.New(db)
	err := ste.GetById(siteid)
	if err != nil {
		err := errors.New("Failed to get site")
		http.Fail(c, 500, err.Error(), err)
		return
	}

	// Decode digest
	digest := &netlify.Digest{}
	err = json.Decode(c.Request.Body, digest)
	if err != nil {
		err := errors.New("Failed to decode digest")
		http.Fail(c, 500, err.Error(), err)
	}

	// Get access token for organization
	accessToken := netlify.GetAccessToken(ctx, org.Name)

	// Create deploy
	client := netlify.New(ctx, accessToken)
	deploy, err := client.CreateDeploy(ste.Netlify(), digest, false)

	deploy.SiteId = siteid // Override netlify's site id with ours

	if err != nil {
		http.Fail(c, 500, "Failed to create deploy", err)
		return
	}

	http.Render(c, 201, deploy)
}

func getDeploy(c *context.Context) {
}

func listDeploys(c *context.Context) {
}

func restoreDeploy(c *context.Context) {
}
