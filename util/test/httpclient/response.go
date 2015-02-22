package httpclient

import (
	"io/ioutil"
	"net/http"
)

type Response struct {
	*http.Response
}

// Returns body as a string
func (r *Response) Text() (body string, err error) {
	defer r.Response.Body.Close()

	if bytes, err := ioutil.ReadAll(r.Response.Body); err != nil {
		return "", err
	} else {
		body = string(bytes)
	}

	return body, err
}
