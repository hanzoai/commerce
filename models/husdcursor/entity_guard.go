package husdcursor

import "github.com/hanzoai/commerce/models/mixin"

// Compile-time guard: HUSDCursor must satisfy mixin.Entity (see husdissuance).
var _ mixin.Entity = (*HUSDCursor)(nil)
