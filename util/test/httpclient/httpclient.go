package httpclient

import "golang.org/x/net/context"

func New(ctx context.Context, moduleName string) *Client {
	client := new(Client)
	client.context = ctx
	client.moduleName = moduleName
	client.determineBaseURL()
	return client
}
