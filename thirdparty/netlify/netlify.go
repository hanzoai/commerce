package netlify

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/middleware"
	"crowdstart.com/models/site"
	"crowdstart.com/util/log"

	"appengine/urlfetch"
)

func CreateSite(c *gin.Context, s site.Site) {
	ctx := middleware.GetAppEngine(c)
	jsonreq, _ := json.Marshal(s)
	reqbuf := bytes.NewBuffer(jsonreq)
	req, err := http.NewRequest("POST", config.Netlify.BaseUrl+"sites/", reqbuf)

	if err != nil {
		log.Error("Error upon creating new request %v", err, ctx)
		return
	}

	client := urlfetch.Client(ctx)
	res, err := client.Do(req)
	defer res.Body.Close()

	if err != nil {
		log.Error("Request came back with error %v", err, ctx)
		return
	}
}
