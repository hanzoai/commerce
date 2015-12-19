package netlify

import (
	"net/url"

	"github.com/netlify/netlify-go"
)

func (c *Client) CreateDeploy(ste *Site, digest *Digest, draft bool) (*Deploy, error) {
	// Get site for deploy
	_, res, err := c.client.Sites.Get(ste.Id)
	logger(c.ctx)(res, err)

	if err != nil {
		return &Deploy{}, err
	}

	// Manually construct call to netlify for deploy
	params := url.Values{}
	if draft {
		params["draft"] = []string{"true"}
	}
	options := &netlify.RequestOptions{JsonBody: digest, QueryParams: &params}
	ndeploy := &netlify.Deploy{}
	res, err = c.client.Request("POST", "/deploys", options, ndeploy)
	logger(c.ctx)(res, err)

	if err != nil {
		return &Deploy{}, err
	}

	return newDeploy(ndeploy), err
}
