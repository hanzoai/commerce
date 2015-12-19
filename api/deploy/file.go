package site

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/site"
	"crowdstart.com/thirdparty/netlify"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
)

func listFiles(c *gin.Context) {
}

func getFile(c *gin.Context) {
}

func putFile(c *gin.Context) {
	siteid := c.Param("siteid") // oursiteid
	deployid := c.Param("deployid")
	filepath := c.Param("filepath")

	db := datastore.New(c)
	ste := site.New(db)
	err := ste.GetById(siteid)
	if err != nil {
		msg := fmt.Sprintf("Site '%s' not found", siteid)
		http.Fail(c, 404, msg, nil)
		return
	}

	ctx := middleware.GetAppEngine(c)
	org := middleware.GetOrganization(c)
	accessToken := netlify.GetAccessToken(ctx, org.Name)

	url := config.Netlify.BaseUrl + "/sites/" + ste.Netlify().Id + "/deploys/" + deployid + "/" + filepath
	url += "?access_token=" + accessToken
	log.Debug("Returning redirect, upload file to: %s", url, c)
	c.Redirect(307, url)
}
