// Copyright © 2026 Hanzo AI. MIT License.

package organization

import "errors"

// ErrSecretLikeName is returned when an untrusted, gateway-supplied org name is
// actually a raw API key / bearer token rather than a real org identifier.
//
// The predicate itself is secret.Like. It lives in package secret because the
// same list of credential markers also generates the DB backstop trigger on
// _entities — one definition behind both gates, so neither can drift out from
// under the other.
var ErrSecretLikeName = errors.New("organization: refusing to provision org from a bearer-shaped name")
