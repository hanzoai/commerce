package netlify

import (
	"io/ioutil"
	"time"

	"appengine"
	"appengine/urlfetch"

	"crowdstart.com/config"
	"crowdstart.com/util/log"

	"github.com/netlify/netlify-go"
)

type Client struct {
	ctx    appengine.Context
	client *netlify.Client
	token  string
}

func New(ctx appengine.Context, token string) *Client {
	httpclient := urlfetch.Client(ctx)
	httpclient.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(20) * time.Second, // Update deadline to 20 seconds
	}

	log.Debug("Created Netlify client using AccessToken: '%s'", config.Netlify.AccessToken, ctx)

	c := new(Client)
	c.ctx = ctx
	c.token = token
	c.client = netlify.NewClient(&netlify.Config{
		AccessToken: token,
		HttpClient:  httpclient,
		UserAgent:   "Crowdstart/1.0",
	})
	return c
}

func (c *Client) CreateSite(s Site) (*Site, error) {
	log.Debug("Creating site in netlify: %v", s, c.ctx)
	// Create new site on Netlify's side
	nsite, res, err := c.client.Sites.Create(&netlify.SiteAttributes{
		Name: s.Name,
	})
	defer res.Body.Close()

	b, _ := ioutil.ReadAll(res.Body)
	log.Debug("Response from netlify (%v): %v", res.StatusCode, string(b), c.ctx)

	if err != nil {
		log.Error("Failed to create site: %v", err, c.ctx)
	} else {
		log.Error("Created site: %v", nsite, c.ctx)
	}

	return (*Site)(nsite), err
}

func (c *Client) GetSite(siteId string) (*Site, error) {
	nsite, res, err := c.client.Sites.Get(siteId)
	defer res.Body.Close()

	log.Debug("Response from netlify: %v", res, c.ctx)

	return (*Site)(nsite), err
}

// func ListSites(ctx appengine.Context) ([]Site, error) {
// 	client := createClient(ctx)

// 	// Create new site on Netlify's side
// 	nsites, _, err := client.Sites.List(&netlify.ListOptions{})

// 	return nsites, err
// }

func (c *Client) UpdateSite(s Site) (*Site, error) {
	nsite, res, err := c.client.Sites.Get(s.Id)
	defer res.Body.Close()

	log.Debug("Response from netlify: %v", res, c.ctx)

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
	nsite, res, err := c.client.Sites.Get(s.Id)
	defer res.Body.Close()

	log.Debug("Response from netlify: %v", res, c.ctx)

	if err != nil {
		return err
	}

	_, err = nsite.Destroy()

	return err
}
