package netlify

import (
	"crowdstart.com/util/log"

	"github.com/netlify/netlify-go"
)

func (c *Client) CreateSite(s *Site) (*Site, error) {
	// Create new site on Netlify's side
	nsite, res, err := c.client.Sites.Create(&netlify.SiteAttributes{
		Name: s.Name,
	})

	logger(c.ctx)(res, err)

	if err != nil {
		return newSite(nsite), err
	}

	log.Debug("Created site: %v", nsite, c.ctx)
	return newSite(nsite), err
}

func (c *Client) GetSite(siteId string) (*Site, error) {
	nsite, res, err := c.client.Sites.Get(siteId)

	logger(c.ctx)(res, err)

	if err != nil {
		return newSite(nsite), err
	}

	return newSite(nsite), err
}

func (c *Client) UpdateSite(s *Site) (*Site, error) {
	nsite, res, err := c.client.Sites.Get(s.Id)

	logger(c.ctx)(res, err)

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

func (c *Client) DeleteSite(s *Site) error {
	nsite, res, err := c.client.Sites.Get(s.Id)

	logger(c.ctx)(res, err)

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
