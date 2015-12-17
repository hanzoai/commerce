package netlify

import (
	"net/http"
	"time"

	"appengine"
	"appengine/urlfetch"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
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

// Get a client for netlify
func NewFromNamespace(ctx appengine.Context, orgName string) *Client {
	db := datastore.New(ctx)

	// Try to switch back to root namespace
	db.SetNamespace("")

	// Get organization
	org := organization.New(db)
	if err := org.GetById(orgName); err != nil {
		log.Error("Unable to get organization '%s': %v", orgName, err, ctx)
	}

	// Get access token if we don't have one
	if org.Netlify.AccessToken == "" {
		client := New(ctx, config.Netlify.AccessToken)
		user, err := client.AccessToken(org.Id(), org.Name)
		if err != nil {
			log.Error("Unable to get Netlify Access Token: %v", err, ctx)
		}

		org.Netlify.AccessToken = user.AccessToken
		org.Netlify.CreatedAt = user.CreatedAt
		org.Netlify.Email = user.Email
		org.Netlify.Id = user.Id
		org.Netlify.Uid = user.Uid

		if err := org.Put(); err != nil {
			log.Error("Unable to save organiation with Netlify Access Token: %v", err, ctx)
		}
	}

	return New(ctx, org.Netlify.AccessToken)
}
