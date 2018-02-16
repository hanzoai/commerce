package sentry

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"google.golang.org/appengine"
	"google.golang.org/appengine/delay"
	"google.golang.org/appengine/urlfetch"

	"github.com/getsentry/raven-go"

	"hanzo.io/config"
)

const userAgent = "appengine-go-raven/1.0"

func init() {
	gob.Register(raven.Exception{})
	gob.Register(raven.StacktraceFrame{})
}

type SerializedException struct {
	Exception raven.Exception
	Frames    []raven.StacktraceFrame
}

// Copied verbatim from raven-go
func serializedPacket(packet *raven.Packet) (r io.Reader, contentType string) {
	packetJSON, err := packet.JSON()
	if err != nil {
		panic(err)
	}

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
	ctx context.Context
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

func NewClient(ctx context.Context) (client *raven.Client, err error) {
	// NOTE: Creates a weird worker thread processing buffer of requests, we'll close
	// immediately after capturing this packet.
	client, err = raven.NewClient(config.SentryDSN, map[string]string{})
	if err != nil {
		ctx.Errorf("Unable to create Sentry client: %v, %v", client, err)
		return client, err
	}

	// Replace default net/http transport with our app engine transport.
	client.Transport = &AppEngineTransport{ctx}
	return client, err
}

// Create raven.Exception from error
func NewException(err error) SerializedException {
	exc := raven.NewException(err, raven.NewStacktrace(9, 3, nil))
	return serializeException(exc)
}

// Create raven.Exception from string stack
func NewExceptionFromStack(stack string) SerializedException {
	lines := strings.Split(stack, "\n")
	err := errors.New(lines[0])
	exc := raven.NewException(err, raven.NewStacktrace(2, 3, nil))
	return serializeException(exc)
}

// Serialize exception into something that can be gob encoded
func serializeException(exception *raven.Exception) SerializedException {
	numFrames := len(exception.Stacktrace.Frames)
	exc := SerializedException{}
	exc.Exception = *exception
	exc.Frames = make([]raven.StacktraceFrame, numFrames)

	for i := 0; i < numFrames; i++ {
		exc.Frames[i] = *exception.Stacktrace.Frames[i]
	}
	return exc
}

// Deserialized our specialized SerializedException type
func deserializeException(exception SerializedException) *raven.Exception {
	numFrames := len(exception.Frames)
	exc := &exception.Exception
	exc.Stacktrace = &raven.Stacktrace{}
	exc.Stacktrace.Frames = make([]*raven.StacktraceFrame, numFrames)

	var b bool

	for i := 0; i < numFrames; i++ {
		frame := &exception.Frames[i]
		frame.InApp = b
		exc.Stacktrace.Frames[i] = frame
	}
	return exc
}

var CaptureException = delay.Func("sentry-capture-exception", func(ctx context.Context, requestURI string, serialized SerializedException) {
	client, err := NewClient(ctx)
	if err != nil {
		return
	}

	// Send request
	flags := map[string]string{
		"endpoint": requestURI,
	}

	// Capture error
	exc := deserializeException(serialized)
	packet := raven.NewPacket(exc.Value, exc)
	client.Capture(packet, flags)

	// Destroy client
	client.Close()
})
