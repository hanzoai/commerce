package tasks

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine/delay"
	"google.golang.org/appengine/urlfetch"

	"hanzo.io/datastore"
	"hanzo.io/models/webhook"
	"hanzo.io/util/log"
)

type Client struct {
	ctx    context.Context
	client *http.Client
}

func (c *Client) Post(url string, data []byte) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Hanzo/1.0")
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

func createClient(ctx context.Context) *Client {
	client := urlfetch.Client(ctx)
	timeout := time.Duration(20) * time.Second
	ctx, _ = context.WithTimeout(ctx, timeout)
	client.Transport = &urlfetch.Transport{
		Context: ctx,
	}
	return &Client{ctx: ctx, client: client}
}

// Fire webhooks
var Emit = delay.Func("webhook-emit", func(ctx context.Context, org string, event string, data []byte) {
	log.Debug("Emitting webhook '%s' for %s: %s", event, org, data, ctx)

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
