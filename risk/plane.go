// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/util/json"
)

// Budget is how long an authorization-time score may take. A payment processor
// holds the authorization open for a fixed window and a merchant's checkout for
// less; a scoring plane that has not answered inside this has answered
// "unreachable", and the money plane proceeds on its controls alone. It is a
// var so a deployment can widen it for the slower post-purchase stages, and so
// a test can drive the timeout branch.
var Budget = 250 * time.Millisecond

// ErrNoDecision refuses a label with no decision to attach it to. The scoring
// plane learns by correcting a JUDGEMENT it made; an outcome on an event it
// never saw has nowhere to land, so the money plane records it locally and says
// so rather than inventing a wire call.
var ErrNoDecision = errors.New("risk: an outcome with no decision has nothing to label")

// At returns the scoring plane reachable at base — the fleet's api host, e.g.
// "https://api.hanzo.ai". The paths it calls are the /v1/risk contract and are
// spelled here once.
//
// The call carries the CALLER's identity, forwarded verbatim from the request
// being served. It is never minted here: commerce holds no credential for the
// scoring plane, and a hop that could mint one would let any caller reach any
// tenant's model. A call with no request behind it (a cron, a test) forwards
// nothing and the plane refuses it, which is the honest answer.
func At(base string) Client {
	return &plane{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: Budget}}
}

type plane struct {
	base string
	http *http.Client
}

func (p *plane) Decide(ctx context.Context, ask *Ask) (*Decision, error) {
	var out Decision
	if err := p.post(ctx, "/v1/risk/decide", ask, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *plane) Label(ctx context.Context, label *Label) error {
	if label.Decision == "" {
		return ErrNoDecision
	}
	return p.post(ctx, "/v1/risk/decisions/"+url.PathEscape(label.Decision)+"/label", label, nil)
}

// post is the one hop. It bounds itself with [Budget] independently of the
// caller's context so a long-lived request context cannot lend a payment-time
// score an unbounded wait.
func (p *plane) post(ctx context.Context, path string, in, out any) error {
	ctx, cancel := context.WithTimeout(ctx, Budget)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+path, bytes.NewReader(json.EncodeBytes(in)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	forward(ctx, req)

	res, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Cap the read: a scoring plane answering with a body of unbounded size at
	// authorization time is a denial of service on the payment path.
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("risk: %s answered %d", path, res.StatusCode)
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	return json.DecodeBytes(body, out)
}

// forward copies the caller's identity onto the outbound request. These are the
// gateway's assertion, minted by IAM upstream; a service passes them along and
// cannot write them itself.
func forward(ctx context.Context, req *http.Request) {
	c := zip.CallerOf(ctx)
	set := func(k, v string) {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	set(zip.HeaderOrg, c.Org)
	set(zip.HeaderUser, c.User)
	set(zip.HeaderUserEmail, c.Email)
	set(zip.HeaderRequestID, c.RequestID)
}
