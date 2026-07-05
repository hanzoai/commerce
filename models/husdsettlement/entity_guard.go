package husdsettlement

import "github.com/hanzoai/commerce/models/mixin"

// Compile-time guard: HUSDSettlement must satisfy mixin.Entity (see husdissuance).
var _ mixin.Entity = (*HUSDSettlement)(nil)
