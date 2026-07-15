package site

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/site"
	"github.com/hanzoai/commerce/thirdparty/netlify"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

func createDeploy(c *zip.Ctx) error {
	ctx := c.Context()
	org := middleware.GetOrganization(c)
	siteid := c.Param("siteid")

	// Get associated site
	db := datastore.New(ctx)
	ste := site.New(db)
	err := ste.GetById(siteid)
	if err != nil {
		err := errors.New("Failed to get site")
		return http.Fail(c, 500, err.Error(), err)
	}

	// Decode digest
	digest := &netlify.Digest{}
	err = json.DecodeBytes(c.Body(), digest)
	if err != nil {
		err := errors.New("Failed to decode digest")
		return http.Fail(c, 500, err.Error(), err)
	}

	// Get access token for organization
	accessToken := netlify.GetAccessToken(ctx, org.Name)

	// Create deploy
	client := netlify.New(ctx, accessToken)
	deploy, err := client.CreateDeploy(ste.Netlify(), digest, false)

	deploy.SiteId = siteid // Override netlify's site id with ours

	if err != nil {
		return http.Fail(c, 500, "Failed to create deploy", err)
	}

	return http.Render(c, 201, deploy)
}

func getDeploy(c *zip.Ctx) error {
	return nil
}

func listDeploys(c *zip.Ctx) error {
	return nil
}

func restoreDeploy(c *zip.Ctx) error {
	return nil
}
