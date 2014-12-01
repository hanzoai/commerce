package mandrill

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"appengine"
	"appengine/urlfetch"

	"crowdstart.io/util/log"
)

const apiKey = "wJ3LGLp5ZOUZlSH8wwqmTg"
const root = "http://mandrillapp.com/api/1.0"

// Escapes special chars in the html
func GetHtml(filename string) string {
	wd, _ := os.Getwd()
	log.Info(wd)
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Panic(err.Error())
		return ""
	}

	// http://stackoverflow.com/a/16652683
	o := bytes.NewBufferString("")
	for _, c := range string(b) {
		switch c {
		case '\\', '"':
			o.WriteRune('\\')
			o.WriteRune(c)
		case '\b':
			o.WriteString(`\b`)
		case '\t':
			o.WriteString(`\t`)
		case '\n':
			o.WriteString(`\n`)
		case '\f':
			o.WriteString(`\f`)
		case '\r':
			o.WriteString(`\r`)
		case '%': // For string formatting to not break.
			o.WriteString("%%")
		default:
			o.WriteRune(c)
		}
	}

	return o.String()
}

// PingMandrill checks if our credentials/url are okay
// Returns true if Mandrill replies with  a 200 OK
func Ping(ctx appengine.Context) bool {
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

func SendMail(
	ctx appengine.Context,
	from_name, from_email, to_name, to_email, subject, template string,
	vars map[string]string) error {
	// Convert the map of vars to a byte buffer of a json string
	varsJson := bytes.NewBufferString("[")
	func() {
		i := 0
		for k, v := range vars {
			if i != 0 {
				varsJson.WriteString(",")
			}
			varsJson.WriteString(fmt.Sprintf(`{ "name" : "%s", "content" : "%s"}`, k, v))
			i += 1
		}
		varsJson.WriteString("]")
	}()

	url := root + "/messages/send.json"
	log.Debug(url)

	j := fmt.Sprintf(`
{
    "key": "%s",
    "template_name": "Preorder confirmation",
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
        "important": false,
        "track_opens": true,
        "track_clicks": true,
        "preserve_recipients": true,
        "merge": true,
        "merge_language": "mailchimp",
        "merge_vars": [
            {
                "rcpt": "%s",
                "vars": %s
            }
        ]
    },
    "async": true,
    "ip_pool": "Main Pool"
}`,
		apiKey,
		template,
		subject,
		from_email,
		from_name,
		to_email,
		to_name,
		"noreply@skullysystems.com",
		to_email,
		varsJson.String(),
	)
	log.Info(j)

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
