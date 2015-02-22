package httpclient

import "appengine"

func getModuleHost(ctx appengine.Context, moduleName string) (host string, err error) {
	return appengine.ModuleHostname(ctx, moduleName, "", "")
}

func New(ctx appengine.Context, moduleName string) *Client {
	client := new(Client)
	client.context = ctx
	client.moduleName = moduleName
	client.determineBaseURL()
	return client
}
