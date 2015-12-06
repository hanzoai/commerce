package site

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/site"
	"crowdstart.com/thirdparty/netlify"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"github.com/gin-gonic/gin"
)

func createSite(c *gin.Context) {
	db := datastore.New(c)
	s := site.New(db)

	if err := json.Decode(c.Request.Body, s); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}

	netlify.CreateSite(c, s)
}

func updateSite(c *gin.Context) {
	db := datastore.New(c)
	s := site.New(db)
	siteid := c.Param("siteid")
	s.Netlify.Id = siteid
	if err := json.Decode(c.Request.Body, s); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}

	netlify.UpdateSite(c, s)
}

func deleteSite(c *gin.Context) {
	db := datastore.New(c)
	s := site.New(db)
	siteid := c.Param("siteid")
	s.Netlify.Id = siteid
	netlify.DeleteSite(c, s.Netlify.Id)
}

func listSites(c *gin.Context) {
	netlify.ListSites(c)
}

func getSite(c *gin.Context) {
	db := datastore.New(c)
	s := site.New(db)
	siteid := c.Param("siteid")
	s.Netlify.Id = siteid

	netlify.GetSite(c, s.Netlify.Id)
}
