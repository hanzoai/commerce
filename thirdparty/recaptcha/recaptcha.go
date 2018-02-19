package recaptcha

import (
	"io/ioutil"
	"net/url"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"hanzo.io/util/json"
	"hanzo.io/log"
)

type RecaptchaResponse struct {
	Success     bool      `json:"success"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []string  `json:"error-codes"`
}

func Challenge(ctx context.Context, privateKey, response string) bool {
	// log.Warn("Captcha:\n\n%s\n\n%s\n\n%s", privateKey, response, ctx)
	client := urlfetch.Client(ctx)
	r := RecaptchaResponse{}
	resp, err := client.PostForm("https://www.google.com/recaptcha/api/siteverify",
		url.Values{
			"secret":   {privateKey},
			"response": {response},
			// "remoteip": {remoteIp},
		})
	if err != nil {
		log.Error("Captcha post error: %s", err, ctx)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	log.Warn("Captcha %s", body, ctx)
	if err != nil {
		log.Error("Read error: could not read body: %s", err, ctx)
		return false
	}
	err = json.Unmarshal(body, &r)
	log.Warn("Captcha %v", r, ctx)
	if err != nil {
		log.Error("Read error: got invalid JSON: %s", err, ctx)
		return false
	}

	return r.Success
}
