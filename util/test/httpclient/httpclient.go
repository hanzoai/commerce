package httpclient

import (
	"context"

	"google.golang.org/appengine"
)

func getModuleHost(ctx context.Context, moduleName string) (host string, err error) {
	return appengine.ModuleHostname(ctx, moduleName, "", "")
}

func New(ctx context.Context, moduleName string) *Client {
	client := new(Client)
	client.context = ctx
	client.moduleName = moduleName
	client.determineBaseURL()
	return client
}
