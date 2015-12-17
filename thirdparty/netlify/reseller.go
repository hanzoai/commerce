package netlify

import (
	"net/http"
	"time"

	"appengine"
	"appengine/memcache"
	"appengine/urlfetch"

	"crowdstart.com/config"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

type User struct {
	Email       string    `json:"email"`
	Id          string    `json:"id,omitempy"` // Netlify's id for this user
	Uid         string    `json:"uid"`         // Our id for this user
	CreatedAt   time.Time `json:"created_at,omitempy"`
	AccessToken string    `json:"access_token,omitempy"`
}

type AccessTokenReq struct {
	User User `json:"user"`
}

func (c *Client) AccessToken(email, userId string) (User, error) {
	buf := json.EncodeBuffer(AccessTokenReq{User: User{Email: email, Uid: userId}})
	url := config.Netlify.BaseUrl + "access_tokens?access_token=" + config.Netlify.AccessToken
	req, err := http.NewRequest("POST", url, buf)

	user := User{}

	if err != nil {
		log.Error("Error upon creating new request %v", err, c.ctx)
		return user, err
	}

	client := urlfetch.Client(c.ctx)
	res, err := client.Do(req)
	defer res.Body.Close()

	// Decode body
	if err := json.Decode(res.Body, &user); err != nil {
		log.Error("Unable to parse response from Netlify: %v", err, c.ctx)
	}

	if err != nil {
		log.Error("Request came back with error %v", err, c.ctx)
		return user, err
	}

	return user, nil
}

// Get access token for organization out of memcache
func getCachedToken(ctx appengine.Context, orgName string) string {
	if item, err := memcache.Get(ctx, "netlify-access-token"); err == memcache.ErrCacheMiss {
		return ""
	} else if err != nil {
		return ""
	} else {
		return string(item.Value)
	}
}

// Get access token
func getAccessToken(ctx appengine.Context, orgName string) string {
	client := New(ctx, config.Netlify.AccessToken)
	user, err := client.AccessToken(orgName, orgName+"@crowdstart.com")

	if err != nil {
		log.Error("Unable to get Netlify Access Token: %v", err, ctx)
		return ""
	}

	return user.AccessToken
}

// Cache access token
func cacheAccessToken(ctx appengine.Context, accessToken string) {
	item := &memcache.Item{
		Key:   "netlify-access-token",
		Value: []byte(accessToken),
	}

	// Persist to memcache
	if err := memcache.Set(ctx, item); err != nil {
		log.Error("Unable to persist access token: %v", err, ctx)
	}
}

// Get a client for netlify
func NewFromNamespace(ctx appengine.Context, orgName string) *Client {
	log.Debug("Fetching access token for organization from memcached", ctx)
	accessToken := getCachedToken(ctx, orgName)
	if accessToken == "" {
		log.Debug("No access token found, creating new access token.", ctx)
		accessToken = getAccessToken(ctx, orgName)
		log.Debug("Caching access token '%s'", accessToken, ctx)
		cacheAccessToken(ctx, accessToken)
	}

	log.Debug("Creating new client using access token '%s'", accessToken, ctx)
	return New(ctx, accessToken)
}
