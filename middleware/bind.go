// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"context"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/user"
)

// bind.go is the seam between a REQUEST and a TYPED op.
//
// A typed zip handler receives only a context.Context and its decoded input, so
// the two facts every commerce handler needs are not in its hands: the
// resolved ORG, and WHO is asking. Both are set by the auth middleware as
// request locals, and neither may EVER become an input field — an input field
// is caller-supplied, so a tenant read from one is a cross-tenant read the
// caller asserted for itself.
//
// [Bind] parks both on the context; [OrgFrom] and [WhoFrom] read them back.
// Install it on the group, AFTER the token middleware that resolves them and
// BEFORE the typed routes it serves: this router runs middleware in
// registration order, so one installed after its leaves never runs.
//
// FAIL CLOSED OFF THE HTTP PATH. The CLI and op-call projections invoke an op
// with no request behind them, so OrgFrom finds nothing there and the op
// refuses — the handler's own gate, with no second gate to keep in sync.

type boundKey struct{}

// bound is everything Bind carries for one request, under ONE key so the
// middleware costs one allocation however many facts have to cross.
type bound struct {
	org *organization.Organization
	who string
}

// Bind carries the request's tenant and principal into the typed ops beneath
// it. A request that arrives with no organization resolved is bound with none,
// and the ops refuse — Bind never invents a tenant.
func Bind() zip.Handler {
	return func(c *zip.Ctx) error {
		b := &bound{}
		if org, ok := GetOrganizationOK(c); ok && org != nil {
			b.org = org
		}
		if u, ok := c.Locals("user").(*user.User); ok && u != nil {
			b.who = u.Id()
		}
		ctx := context.WithValue(c.Context(), boundKey{}, b)
		if b.org != nil {
			// The namespaced context IS the tenant boundary for every store
			// read a typed op makes: a datastore built from it can only see
			// this org's rows.
			ctx = b.org.Namespaced(ctx)
		}
		c.SetContext(ctx)
		return c.Continue()
	}
}

// OrgFrom returns the organization a typed op is serving. Absent means the
// caller was not resolved to a tenant, and the op must refuse rather than
// choose one.
func OrgFrom(ctx context.Context) (*organization.Organization, bool) {
	b, ok := ctx.Value(boundKey{}).(*bound)
	if !ok || b.org == nil {
		return nil, false
	}
	return b.org, true
}

// WhoFrom returns the validated principal's user id, or empty when the request
// carried none. It is the ONLY source of authorship for a record: a control's
// author or an outcome's reporter taken off the wire is not an audit trail.
func WhoFrom(ctx context.Context) string {
	b, ok := ctx.Value(boundKey{}).(*bound)
	if !ok {
		return ""
	}
	return b.who
}
