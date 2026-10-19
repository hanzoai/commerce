// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package auth

import "testing"

// The `owner` claim is the org of the APPLICATION a token was minted through, not
// the subject's, so it is never SuperAdmin on its own — in any case or padding.
func TestIAMClaims_OwnerClaimIsNotSuperAdmin(t *testing.T) {
	for _, owner := range []string{"admin", "Admin", "ADMIN", " admin "} {
		if (&IAMClaims{Owner: owner}).IsSuperAdmin() {
			t.Errorf("owner claim %q read as SuperAdmin", owner)
		}
	}
}
