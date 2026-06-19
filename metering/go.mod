// github.com/hanzoai/commerce/metering is a standalone, stdlib-only module so
// any product can import the usage-metering hook without pulling in the
// commerce server's dependency graph. It is nested in the commerce repo (the
// billing source of truth) but versions and resolves independently.
//
// go 1.24 (not the repo's 1.26) keeps the import compatible with every Hanzo
// service regardless of its toolchain — products never bump Go to adopt billing.
module github.com/hanzoai/commerce/metering

go 1.24
