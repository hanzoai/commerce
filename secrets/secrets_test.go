// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package secrets

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zap-proto/zip"
)

// store is a host secret plane that keeps cloud's tenancy rule (apps/kms admit): a
// ref naming orgs/<org>/… is served only to a call whose stated caller acts for
// that org. It records every ref it was asked for.
type store struct {
	Values map[string]string
	Asked  []string
}

var errRefused = errors.New("kms: ref belongs to another org than the call acts for")

func (s *store) GetSecret(ctx context.Context, ref string) ([]byte, error) {
	s.Asked = append(s.Asked, ref)
	if rest, ok := strings.CutPrefix(ref, "orgs/"); ok {
		org, _, _ := strings.Cut(rest, "/")
		if zip.CallerOf(ctx).Org != org {
			return nil, errRefused
		}
	}
	if v, ok := s.Values[ref]; ok {
		return []byte(v), nil
	}
	return nil, errors.New("no such secret")
}

func install(t *testing.T, s *store) {
	t.Helper()
	prev := Get()
	Set(s)
	t.Cleanup(func() { Set(prev) })
}

// Org reads orgs/<org>/<name> and states org on the call, so a store that serves
// a tenant's material only to that tenant answers it — even when the request
// behind ctx was sent by someone in another org, which is the payer paying a
// brand.
func TestOrgReadsAsTheOrg(t *testing.T) {
	s := &store{Values: map[string]string{"orgs/hanzo/wire/WIRE_SWIFT": "BOFAUS3N"}}
	install(t, s)

	payer := zip.WithCaller(context.Background(), zip.Caller{Org: "acme", User: "alice", OrgAdmin: true})
	if got := Org(payer, "hanzo", "wire/WIRE_SWIFT"); got != "BOFAUS3N" {
		t.Fatalf("Org = %q, asked %v; want the brand's value read as the brand", got, s.Asked)
	}
	if len(s.Asked) != 1 || s.Asked[0] != "orgs/hanzo/wire/WIRE_SWIFT" {
		t.Fatalf("asked %v, want exactly orgs/hanzo/wire/WIRE_SWIFT (no leading slash: a tenant ref)", s.Asked)
	}
	// The same store refuses the payer's own context, which is why Org states one.
	if _, err := s.GetSecret(payer, "orgs/hanzo/wire/WIRE_SWIFT"); err == nil {
		t.Fatal("the fake store does not keep the tenancy rule, so this test proves nothing")
	}
}

// A caller from another org is re-pointed with zip.ActingAs, so the read names
// who is acting; a caller already in the org is left as it is; with no caller,
// nothing is read — cloud's KMS would answer an unstated call as its own brand.
func TestAsRecordsWhoIsActing(t *testing.T) {
	payer := zip.WithCaller(context.Background(), zip.Caller{Org: "acme", User: "alice", Project: "p1"})
	actx, err := as(payer, "hanzo")
	if err != nil {
		t.Fatal(err)
	}
	if who := zip.CallerOf(actx); who.Org != "hanzo" || who.ActedBy != "alice" || who.Project != "" {
		t.Fatalf("caller = %+v, want org hanzo acted by alice, no project", who)
	}

	same := zip.WithCaller(context.Background(), zip.Caller{Org: "hanzo", User: "z"})
	if actx, err := as(same, "hanzo"); err != nil || zip.CallerOf(actx).ActedBy != "" {
		t.Fatalf("a caller already in the org was re-pointed: %+v, %v", zip.CallerOf(actx), err)
	}

	if _, err := as(context.Background(), "hanzo"); err == nil {
		t.Fatal("a call with no caller was given an org to act for")
	}
	s := &store{Values: map[string]string{"orgs/hanzo/wire/WIRE_SWIFT": "BOFAUS3N"}}
	install(t, s)
	if got := Org(context.Background(), "hanzo", "wire/WIRE_SWIFT"); got != "" || len(s.Asked) != 0 {
		t.Fatalf("a call with no caller read %q (asked %v)", got, s.Asked)
	}
}

func TestOrgRefusesWhatCannotFormARef(t *testing.T) {
	s := &store{Values: map[string]string{}}
	install(t, s)
	for _, c := range []struct{ org, name string }{
		{"", "wire/X"}, {"hanzo", ""}, {"a/b", "wire/X"}, {"hanzo@prod", "wire/X"}, {"hanzo", "wire/X@prod"},
		{" hanzo", "wire/X"}, {"hanzo ", "wire/X"}, {"Hanzo", "wire/X"}, {"team.a", "wire/X"}, {"hanzo", "/wire/X"},
	} {
		if got := Org(context.Background(), c.org, c.name); got != "" {
			t.Errorf("Org(%q, %q) = %q, want refused", c.org, c.name, got)
		}
	}
	if len(s.Asked) != 0 {
		t.Fatalf("a malformed ref reached the store: %v", s.Asked)
	}
}

func TestOrgWithNoReaderReadsNothing(t *testing.T) {
	prev := Get()
	Set(nil)
	t.Cleanup(func() { Set(prev) })
	if got := Org(context.Background(), "hanzo", "wire/X"); got != "" {
		t.Fatalf("Org with no reader = %q, want empty", got)
	}
}
