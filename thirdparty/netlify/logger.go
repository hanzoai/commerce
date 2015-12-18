package netlify

import (
	"io/ioutil"

	"appengine"

	"github.com/netlify/netlify-go"

	"crowdstart.com/util/log"
)

type netlifyLogger func(*netlify.Site, *netlify.Response, error) (*netlify.Site, *netlify.Response, error)

func logger(ctx appengine.Context) netlifyLogger {
	return func(site *netlify.Site, res *netlify.Response, err error) (*netlify.Site, *netlify.Response, error) {
		if err != nil {
			return site, res, err
		}

		defer res.Body.Close()
		b, _ := ioutil.ReadAll(res.Body)
		log.Debug("Netlify Response (%v): %v", res.StatusCode, string(b), ctx)
		return site, res, err
	}
}
