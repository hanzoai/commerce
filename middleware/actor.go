// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package middleware

import (
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware/iammiddleware"
)

// Actor names the principal behind a privileged act — a mint, a comp, a balance
// adjustment, a platform coupon edit — for the row or audit record it writes.
// Every such route admits only the platform principal (MayMintMoney /
// RequirePlatformAdmin: owner "admin"), whose validated claims name them: the
// email, else owner/name, else the subject, else the owner.
func Actor(c *zip.Ctx) string {
	claims := iammiddleware.GetIAMClaims(c)
	if e := strings.TrimSpace(claims.Email); e != "" {
		return e
	}
	if n := strings.TrimSpace(claims.Name); n != "" {
		return strings.TrimSpace(claims.Owner) + "/" + n
	}
	if s := strings.TrimSpace(claims.Subject); s != "" {
		return s
	}
	return strings.TrimSpace(claims.Owner)
}
