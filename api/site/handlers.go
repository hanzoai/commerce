package site

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/site"
	"crowdstart.com/thirdparty/netlify"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"github.com/gin-gonic/gin"
)

func create(c *gin.Context) {
	db := datastore.New(c)
	s := site.New(db)
	if err := json.Decode(c.Request.Body, s); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}

	netlify.CreateSite(c, s)
}

func update(c *gin.Context) {
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

func delete(c *gin.Context) {
	db := datastore.New(c)
	s := site.New(db)
	siteid := c.Param("siteid")
	s.Netlify.Id = siteid
	netlify.DeleteSite(c, s)
}

func list(c *gin.Context) {
	netlify.GetAllSites(c)
}

func get(c *gin.Context) {
	db := datastore.New(c)
	s := site.New(db)
	siteid := c.Param("siteid")
	s.Netlify.Id = siteid

	netlify.GetSingleSite(c, s)

}
