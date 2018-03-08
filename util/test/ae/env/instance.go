package env

import (
	"io"
	"net/http"

	"github.com/zeekay/aetest"
)

type Instance struct {
	Ctx  *FancyContext
	Inst aetest.Instance
}

func (i Instance) Close() error {
	return Close(i.Ctx)
}

func (i Instance) NewRequest(method, urlStr string, body io.Reader) (*http.Request, error) {
	return i.Inst.NewRequest(method, urlStr, body)
}
