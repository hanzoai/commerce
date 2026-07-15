package site

import (
	"github.com/zap-proto/zip"

	// "github.com/hanzoai/commerce/datastore"
	// "github.com/hanzoai/commerce/models/site"
	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/thirdparty/netlify"
)

func listFiles(c *zip.Ctx) error {
	return nil
}

func getFile(c *zip.Ctx) error {
	return nil
}

func putFile(c *zip.Ctx) error {
	// siteid := c.Param("siteid") // oursiteid
	deployid := c.Param("deployid")
	filepath := c.Param("filepath")

	// db := datastore.New(c)
	// ste := site.New(db)
	// err := ste.GetById(siteid)
	// if err != nil {
	// 	msg := fmt.Sprintf("Site '%s' not found", siteid)
	// 	http.Fail(c, 404, msg, nil)
	// 	return
	// }

	ctx := c.Context()
	org := middleware.GetOrganization(c)
	accessToken := netlify.GetAccessToken(ctx, org.Name)

	url := config.Netlify.BaseUrl + "/deploys/" + deployid + "/files" + filepath
	url += "?access_token=" + accessToken
	log.Debug("Returning redirect, upload file to: %s", url, c)
	return c.Redirect(307, url)
}
