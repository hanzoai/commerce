package depositcursor

import "github.com/hanzoai/commerce/models/mixin"

// Compile-time guard: DepositCursor must satisfy mixin.Entity (see husdcursor).
var _ mixin.Entity = (*DepositCursor)(nil)
