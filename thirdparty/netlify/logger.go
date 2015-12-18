package netlify

import (
	"io/ioutil"

	"appengine"

	"github.com/netlify/netlify-go"

	"crowdstart.com/util/log"
)

func logger(ctx appengine.Context) func(*netlify.Response, error) {
	return func(res *netlify.Response, err error) {
		if err != nil {
			return
		}

		defer res.Body.Close()
		b, _ := ioutil.ReadAll(res.Body)
		log.Debug("Netlify Response (%v): %v", res.StatusCode, string(b), ctx)
	}
}
