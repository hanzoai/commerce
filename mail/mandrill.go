package mail

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"appengine"
	"appengine/urlfetch"

	"crowdstart.io/util/log"
)

const apiKey = "wJ3LGLp5ZOUZlSH8wwqmTg"
const root = "http://mandrillapp.com/api/1.0"

var html = func() string {
	wd, _ := os.Getwd()
	log.Info(wd)
	b, err := ioutil.ReadFile("../../resources/confirmation_email.html")
	if err != nil {
		log.Panic(err.Error())
		return ""
	}

	s := strings.Replace(string(b), `"`, `\"`, -1)
	return s
}()

// PingMandrill checks if our credentials/url are okay
// Returns true if Mandrill replies with  a 200 OK
func PingMandrill(ctx appengine.Context) bool {
	url := root + "/users/ping.json"
	log.Debug(url)

	str := fmt.Sprintf(`{"key": "%s"}`, apiKey)
	log.Debug(str)
	body := []byte(str)
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

	log.Debug(res.Status)
	return res.StatusCode == 200
}

func SendMail(ctx appengine.Context, from_name, from_email, to_name, to_email, subject string) error {
	url := root + "/messages/send.json"
	log.Debug(url)

	j := fmt.Sprintf(`{
    "key": "%s",
    "message": {
        "html": "%s",
        "subject": "%s",
        "from_email": "%s",
        "from_name": "%s",
        "to": [
            {
                "email": "%s",
                "name": "%s",
                "type": "to"
            }
        ],
        "headers": {
            "Reply-To": "%s"
        },
        "important": true,
        "track_opens": true,
        "track_clicks": true,
        "auto_text": null,
        "auto_html": null,
        "inline_css": "",
        "url_strip_qs": "",
        "preserve_recipients": "",
        "view_content_link": "",
        "merge_language": "mailchimp",
        "tags": [
            "skully, preorder"
        ]
    },
    "async": true,
    "ip_pool": "Main Pool"
}`,
		apiKey,
		html,
		subject,
		from_email,
		from_name,
		to_email,
		to_name,
		"noreply@skullysystems.com",
	)
	log.Debug(j)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(j)))
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

	if res.StatusCode == 200 {
		return nil
	} else {
		b, _ := ioutil.ReadAll(res.Body)
		log.Debug(string(b))
		log.Debug(apiKey)
		return errors.New("Email not sent")
	}
}
