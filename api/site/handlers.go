package site

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/site"
	"crowdstart.com/thirdparty/netlify"
	"crowdstart.com/util/json"
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
	if err := json.Decode(c.Request.Body, s); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}
}

func destroySite(c *gin.Context) {
	db := datastore.New(c)
	s := site.New(db)
	if err := json.Decode(c.Request.Body, s); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}
}

func getAllSites(c *gin.Context) {
	db := datastore.New(c)
	s := site.New(db)
	if err := json.Decode(c.Request.Body, s); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}
}

func getSingleSite(c *gin.Context) {
	db := datastore.New(c)
	s := site.New(db)
	if err := json.Decode(c.Request.Body, s); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}
}
