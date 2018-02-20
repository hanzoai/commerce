package httpclient

import (
	"context"
)

func getModuleHost(ctx context.Context, moduleName string) (host string, err error) {
	// return appengine.ModuleHostname(ctx, moduleName, "", "")
	return "localhost:8000", nil
}

func New(ctx context.Context, moduleName string) *Client {
	client := new(Client)
	client.context = ctx
	client.moduleName = moduleName
	client.determineBaseURL()
	return client
}
