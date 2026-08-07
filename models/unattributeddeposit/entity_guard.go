package unattributeddeposit

import "github.com/hanzoai/commerce/models/mixin"

// Compile-time guard: UnattributedDeposit must satisfy mixin.Entity (see
// depositcursor).
var _ mixin.Entity = (*UnattributedDeposit)(nil)
