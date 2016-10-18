package tasks

import (
	"io/ioutil"
	"net/http"
	"time"

	"appengine"
	"appengine/delay"
	"appengine/urlfetch"

	"crowdstart.com/datastore"
	"crowdstart.com/models/webhook"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

type Client struct {
	ctx    appengine.Context
	client *http.Client
}

func (c *Client) Post(url string, data interface{}) error {
	req, err := http.NewRequest("POST", url, json.EncodeBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Crowdstart/1.0")
	req.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Read response
	body, _ := ioutil.ReadAll(res.Body)
	log.Debug("Webhook endpoint '%s' responded with %v", url, body)

	return nil
}

func createClient(ctx appengine.Context) *Client {
	client := urlfetch.Client(ctx)
	client.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(20) * time.Second, // Update deadline to 20 seconds
	}

	return &Client{ctx: ctx, client: client}
}

// Fire webhooks
var Emit = delay.Func("webhook-emit", func(ctx appengine.Context, org string, event string, data interface{}) {
	log.Debug("Emit webhook '%s' for '%s'", event, org, ctx)
	log.JSON(data)

	db := datastore.New(ctx)
	db.SetNamespace(org)

	// Fetch any webhooks for this organization
	hooks, err := webhook.Query(db).GetEntities()
	if err != nil {
		log.Warn("Failed to retrieve webhooks for organization '%s': %v", org, err, ctx)
	}

	// No hooks! Bye!
	if len(hooks) == 0 {
		log.Debug("No webhooks defined for organization '%s'", org, ctx)
		return
	}

	// Create client to send event data
	client := createClient(ctx)

	for i := range hooks {
		hook := hooks[i].(*webhook.Webhook)

		// Has all events enabled
		if hook.All {
			client.Post(hook.Url, data)
			continue
		}

		// Check if current event is enabled
		if enabled, ok := hook.Events[event]; ok && enabled {
			client.Post(hook.Url, data)
		}
	}
})
