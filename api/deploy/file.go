package site

import (
	"github.com/gin-gonic/gin"

	// "hanzo.io/datastore"
	// "hanzo.io/models/site"
	"hanzo.io/config"
	"hanzo.io/middleware"
	"hanzo.io/thirdparty/netlify"
	"hanzo.io/util/log"
)

func listFiles(c *context.Context) {
}

func getFile(c *context.Context) {
}

func putFile(c *context.Context) {
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

	ctx := middleware.GetAppEngine(c)
	org := middleware.GetOrganization(c)
	accessToken := netlify.GetAccessToken(ctx, org.Name)

	url := config.Netlify.BaseUrl + "/deploys/" + deployid + "/files" + filepath
	url += "?access_token=" + accessToken
	log.Debug("Returning redirect, upload file to: %s", url, c)
	c.Redirect(307, url)
}
