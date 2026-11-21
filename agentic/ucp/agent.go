// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"net/url"
	"strings"
)

// ParseAgent reads the UCP-Agent header: an RFC 8941 Dictionary whose required
// profile member is a quoted string holding an HTTPS URL (signatures,
// "UCP-Agent parsing rules"; overview, "Platform Advertisement on Request").
// A header that does not parse, lacks profile, or names a URL that is not HTTPS
// is invalid_profile_url.
func ParseAgent(h string) (string, *Error) {
	d, err := parseDict(h)
	if err != nil {
		return "", fail(CodeInvalidProfileURL, "UCP-Agent is not a structured field dictionary: %v", err)
	}
	v, ok := d.get("profile")
	if !ok {
		return "", fail(CodeInvalidProfileURL, "UCP-Agent has no profile member")
	}
	it, ok := v.(item)
	s, isStr := it.value.(string)
	if !ok || !isStr {
		return "", fail(CodeInvalidProfileURL, "UCP-Agent profile must be a quoted string")
	}
	if err := checkURL(s); err != nil {
		return "", fail(CodeInvalidProfileURL, "UCP-Agent profile: %v", err)
	}
	return s, nil
}

// checkURL requires an absolute HTTPS URL without userinfo and with a host of
// at least two labels, which is not an IP literal (overview, "Derivation
// algorithm", steps 1 and 2).
func checkURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return errNotHTTPS
	}
	if u.User != nil {
		return errUserinfo
	}
	h := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if h == "" || !strings.Contains(h, ".") || isIPLiteral(h) {
		return errHost
	}
	return nil
}
