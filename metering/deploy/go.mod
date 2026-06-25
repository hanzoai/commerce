// github.com/hanzoai/commerce/metering/deploy renders per-tenant operator
// Service CRs that wire prepaid metering onto a commercially-deployable OSS
// Hanzo product. It is stdlib-only (text/template) so the PaaS control plane
// can vendor it without any dependency graph.
module github.com/hanzoai/commerce/metering/deploy

go 1.24
