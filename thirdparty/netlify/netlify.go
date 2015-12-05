package netlify

import (
	"bytes"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/middleware"
	"crowdstart.com/models/site"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"

	"appengine/urlfetch"
)

func CreateSite(c *gin.Context, s *site.Site) {
	ctx := middleware.GetAppEngine(c)
	jsonreq := json.Encode(s)
	reqbuf := bytes.NewBufferString(jsonreq)
	req, err := http.NewRequest("POST", config.Netlify.BaseUrl+"sites?access_token="+config.Netlify.AccessToken, reqbuf)

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

func UpdateSite(c *gin.Context, s *site.Site) {
	ctx := middleware.GetAppEngine(c)
	jsonreq := json.Encode(s)
	reqbuf := bytes.NewBufferString(jsonreq)
	req, err := http.NewRequest("PUT", config.Netlify.BaseUrl+"sites/"+s.Netlify.Id+"?access_token="+config.Netlify.AccessToken, reqbuf)

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

func DeleteSite(c *gin.Context, s *site.Site) {
	ctx := middleware.GetAppEngine(c)
	req, err := http.NewRequest("DELETE", config.Netlify.BaseUrl+"sites/"+s.Netlify.Id+"?access_token="+config.Netlify.AccessToken, nil)

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

func GetSite(c *gin.Context, s *site.Site) (*site.Site, error) {
	ctx := middleware.GetAppEngine(c)
	req, err := http.NewRequest("GET", config.Netlify.BaseUrl+"sites/"+s.Netlify.Id+"?access_token="+config.Netlify.AccessToken, nil)

	if err != nil {
		log.Error("Error upon creating new request %v", err, ctx)
		return nil, err
	}

	client := urlfetch.Client(ctx)
	res, err := client.Do(req)
	defer res.Body.Close()

	if err != nil {
		log.Error("Request came back with error %v", err, ctx)
		return nil, err
	}

	responseBytes, err := ioutil.ReadAll(res.Body)

	log.Debug("Response Bytes: %v", string(responseBytes), ctx)
	err = json.Unmarshal(responseBytes, s)

	if err != nil {
		log.Error("Could not unmarshal response: %v", err, ctx)
		return nil, err
	}

	return s, nil
}

func ListSites(c *gin.Context) ([]*site.Site, error) {
	ctx := middleware.GetAppEngine(c)
	req, err := http.NewRequest("GET", config.Netlify.BaseUrl+"sites?access_token="+config.Netlify.AccessToken, nil)

	if err != nil {
		log.Error("Error upon creating new request %v", err, ctx)
		return nil, err
	}

	client := urlfetch.Client(ctx)
	res, err := client.Do(req)
	defer res.Body.Close()

	if err != nil {
		log.Error("Request came back with error %v", err, ctx)
		return nil, err
	}

	responseBytes, err := ioutil.ReadAll(res.Body)

	log.Debug("Response Bytes: %v", string(responseBytes), ctx)
	s := []*site.Site{}
	err = json.Unmarshal(responseBytes, s)

	if err != nil {
		log.Error("Could not unmarshal response: %v", err, ctx)
		return nil, err
	}

	return s, nil
}
