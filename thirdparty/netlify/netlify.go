package netlify

import (
	"time"

	"crowdstart.com/config"
	"crowdstart.com/middleware"
	"crowdstart.com/models/site"

	"github.com/gin-gonic/gin"

	"appengine/urlfetch"

	"github.com/netlify/netlify-go"
)

func createClient(c *gin.Context) *netlify.Client {
	ctx := middleware.GetAppEngine(c)

	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(20) * time.Second, // Update deadline to 20 seconds
	}

	return netlify.NewClient(&netlify.Config{
		AccessToken: config.Netlify.AccessToken,
		HttpClient:  httpClient,
		UserAgent:   "Crowdstart/1.0",
	})
}

func CreateSite(c *gin.Context, s *site.Site) error {
	client := createClient(c)

	// Create new site on Netlify's side
	nsite, _, err := client.Sites.Create(&netlify.SiteAttributes{
		Name:         s.Name,
		CustomDomain: s.Domain,
	})

	// Copy over netlify site attributes
	s.Netlify = *nsite

	return err
}

func UpdateSite(c *gin.Context, s *site.Site) error {
	client := createClient(c)

	nsite, _, err := client.Sites.Get(s.Netlify.Id)
	if err != nil {
		return err
	}

	nsite.Url = s.Netlify.Url
	nsite.Name = s.Netlify.Name
	nsite.State = s.Netlify.State
	nsite.UserId = s.Netlify.UserId
	nsite.Premium = s.Netlify.Premium
	nsite.Claimed = s.Netlify.Claimed
	nsite.Password = s.Netlify.Password
	nsite.AdminUrl = s.Netlify.AdminUrl
	nsite.DeployUrl = s.Netlify.DeployUrl
	nsite.CustomDomain = s.Netlify.CustomDomain

	_, err = nsite.Update()
	if err != nil {
		return err
	}
	return nil
}

func DeleteSite(c *gin.Context, siteId string) error {
	client := createClient(c)

	nsite, _, err := client.Sites.Get(siteId)
	if err != nil {
		return err
	}

	_, err = nsite.Destroy()

	return err
}

func GetSite(c *gin.Context, siteId string) (*netlify.Site, error) {
	client := createClient(c)

	nsite, _, err := client.Sites.Get(siteId)

	return nsite, err
}

func ListSites(c *gin.Context) ([]netlify.Site, error) {
	client := createClient(c)

	// Create new site on Netlify's side
	nsites, _, err := client.Sites.List(&netlify.ListOptions{})

	return nsites, err
}
