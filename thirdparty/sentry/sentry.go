package sentry

import (
	"appengine"
	"appengine/delay"
	"appengine/urlfetch"
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/getsentry/raven-go"

	"crowdstart.io/config"
)

const userAgent = "appengine-go-raven/1.0"

// Copied verbatim from raven-go
func serializedPacket(packet *raven.Packet) (r io.Reader, contentType string) {
	packetJSON := packet.JSON()

	// Only deflate/base64 the packet if it is bigger than 1KB, as there is
	// overhead.
	if len(packetJSON) > 1000 {
		buf := &bytes.Buffer{}
		b64 := base64.NewEncoder(base64.StdEncoding, buf)
		deflate, _ := zlib.NewWriterLevel(b64, zlib.BestCompression)
		deflate.Write(packetJSON)
		deflate.Close()
		b64.Close()
		return buf, "application/octet-stream"
	}
	return bytes.NewReader(packetJSON), "application/json"
}

// App Engine transport, uses appengine/urlfetch to deliver packets
type AppEngineTransport struct {
	ctx appengine.Context
}

// Send a packet
func (t *AppEngineTransport) Send(url, authHeader string, packet *raven.Packet) error {
	if url == "" {
		return nil
	}

	client := urlfetch.Client(t.ctx)

	body, contentType := serializedPacket(packet)
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("X-Sentry-Auth", authHeader)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", contentType)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(ioutil.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("raven: got http status %d", res.StatusCode)
	}
	return nil
}

// Do this asynchronously, no need to delay request.
var CaptureException = delay.Func("log-to-sentry", func(ctx appengine.Context, requestURI, stack string) {
	if appengine.IsDevAppServer() {
		return // Don't log to sentry during local development.
	}

	// NOTE: Creates a weird worker thread processing buffer of requests, we'll close
	// immediately after capturing this packet.
	client, err := raven.NewClient(config.SentryDSN, map[string]string{})
	if err != nil {
		ctx.Errorf("Unable to create Sentry client: %v, %v", client, err)
		return
	}

	// Replace default net/http transport with our app engine transport.
	client.Transport = &AppEngineTransport{ctx}

	// Send request
	flags := map[string]string{
		"endpoint": requestURI,
	}

	// Capture error
	packet := raven.NewPacket(stack, raven.NewException(errors.New(stack), raven.NewStacktrace(2, 3, nil)))
	client.Capture(packet, flags)

	// Destroy client
	client.Close()
})
