package netlify

import (
	"time"

	"appengine"

	"crowdstart.com/config"
	"crowdstart.com/util/log"

	"appengine/urlfetch"

	"github.com/netlify/netlify-go"
)

type Client struct {
	client *netlify.Client
}

func New(ctx appengine.Context, token string) *Client {
	client := urlfetch.Client(ctx)
	client.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(20) * time.Second, // Update deadline to 20 seconds
	}

	log.Debug("Created Netlift client using AccessToken: '%s'", config.Netlify.AccessToken, ctx)

	return &Client{netlify.NewClient(&netlify.Config{
		AccessToken: token,
		HttpClient:  client,
		UserAgent:   "Crowdstart/1.0",
	})}
}

func (c *Client) CreateSite(s Site) (*Site, error) {
	// Create new site on Netlify's side
	nsite, _, err := c.client.Sites.Create(&netlify.SiteAttributes{
		Name:         s.Name,
		CustomDomain: s.CustomDomain,
	})

	// Copy over netlify site attributes

	return (*Site)(nsite), err
}

func (c *Client) GetSite(siteId string) (*Site, error) {
	nsite, _, err := c.client.Sites.Get(siteId)

	return (*Site)(nsite), err
}

// func ListSites(ctx appengine.Context) ([]Site, error) {
// 	client := createClient(ctx)

// 	// Create new site on Netlify's side
// 	nsites, _, err := client.Sites.List(&netlify.ListOptions{})

// 	return nsites, err
// }

func (c *Client) UpdateSite(s Site) (*Site, error) {
	nsite, _, err := c.client.Sites.Get(s.Id)
	if err != nil {
		return (*Site)(nsite), err
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
		return (*Site)(nsite), err
	}
	return (*Site)(nsite), nil
}

func (c *Client) DeleteSite(s Site) error {
	nsite, _, err := c.client.Sites.Get(s.Id)
	if err != nil {
		return err
	}

	_, err = nsite.Destroy()

	return err
}
