package netlify

import (
	"appengine"

	"crowdstart.com/util/log"

	"github.com/netlify/netlify-go"
)

type Client struct {
	ctx    appengine.Context
	client *netlify.Client
}

func New(ctx appengine.Context, accessToken string) *Client {
	log.Debug("Created Netlify client using AccessToken: '%s'", accessToken, ctx)

	client := newOauthClient(ctx, accessToken)

	return &Client{
		ctx: ctx,
		client: netlify.NewClient(&netlify.Config{
			AccessToken: accessToken,
			HttpClient:  client,
			UserAgent:   "Crowdstart/1.0",
		}),
	}
}

func (c *Client) CreateSite(s Site) (*Site, error) {
	// Create new site on Netlify's side
	nsite, _, err := logger(c.ctx)(c.client.Sites.Create(&netlify.SiteAttributes{
		Name: s.Name,
	}))

	if err != nil {
		return newSite(nsite), err
	}

	log.Debug("Created site: %v", nsite, c.ctx)
	return newSite(nsite), err
}

func (c *Client) GetSite(siteId string) (*Site, error) {
	nsite, _, err := logger(c.ctx)(c.client.Sites.Get(siteId))

	if err != nil {
		return newSite(nsite), err
	}

	return newSite(nsite), err
}

func (c *Client) UpdateSite(s Site) (*Site, error) {
	nsite, _, err := logger(c.ctx)(c.client.Sites.Get(s.Id))

	if err != nil {
		return newSite(nsite), err
	}

	nsite.Url = s.Url
	nsite.Name = s.Name
	nsite.State = s.State
	nsite.UserId = s.UserId
	nsite.Premium = s.Premium
	nsite.Claimed = s.Claimed
	nsite.Password = s.Password
	nsite.AdminUrl = s.AdminUrl
	nsite.DeployUrl = s.DeployUrl
	nsite.CustomDomain = s.CustomDomain

	_, err = nsite.Update()
	if err != nil {
		return newSite(nsite), err
	}

	return newSite(nsite), nil
}

func (c *Client) DeleteSite(s Site) error {
	nsite, _, err := logger(c.ctx)(c.client.Sites.Get(s.Id))

	if err != nil {
		return err
	}

	_, err = nsite.Destroy()

	return err
}

// func ListSites(ctx appengine.Context) ([]Site, error) {
// 	client := createClient(ctx)

// 	// Create new site on Netlify's side
// 	nsites, _, err := client.Sites.List(&netlify.ListOptions{})

// 	return nsites, err
// }
