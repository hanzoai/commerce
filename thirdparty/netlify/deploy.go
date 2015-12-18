package netlify

func (c *Client) CreateDeploy(s Site) (*Deploy, error) {
	nsite, res, err := c.client.Sites.Get(s.Id)

	logger(c.ctx)(res, err)

	// Create new site on Netlify's side
	ndeploy, _, err := nsite.Deploys.Create("ass")

	return newDeploy(ndeploy), err
	// if err != nil {
	// 	return newSite(nsite), err
	// }

	// log.Debug("Created site: %v", nsite, c.ctx)
	// return newSite(nsite), err
}
