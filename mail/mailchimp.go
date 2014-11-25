package mail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"

	"appengine"
	"appengine/urlfetch"

	"crowdstart.io/util/log"
)

func appengineCtx(c *gin.Context) appengine.Context {
	return appengine.NewContext(c.Request)
}

// Ping is a helper function for checking if our info is correct
func Ping(c *gin.Context) bool {
	url := root + "/helper/ping.json"
	ctx := appengineCtx(c)

	body := []byte(fmt.Sprintf(`{"apikey": "%s", "msg": "ping"}`, apiKey))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		log.Panic(err.Error())
		return false
	}

	client := urlfetch.Client(ctx)
	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		log.Panic(err.Error())
		return false
	}

	return res.StatusCode == 200
}

type Content struct {
	Html string `json:"html"`
	Text string `json:"text"`
}

// CampaignContent returns
func CampaignContent(campaignId string) (Content, error) {
	url := root + "/campaigns/content.json"

	body := []byte(fmt.Sprintf(`{"apikey": "%s", "cid": "%s"}`, apiKey, campaignId))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		log.Panic(err.Error())
		return err
	}

	client := urlfetch.Client(ctx)
	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		log.Panic(err.Error())
		return err
	}

	bodyRes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Panic(err.Error())
		return err
	}

	var content Content
	dec := json.NewDecoder(bytes.NewReader(bodyRes))
	err = dec.Decode(&content)

	return content, err
}
