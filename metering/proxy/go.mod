// github.com/hanzoai/commerce/metering/proxy is the metering reverse proxy —
// the ONE way a non-Go product (vector, search, any HTTP upstream) opts into
// prepaid pay-for-everything. It depends only on the leaf metering client and
// the standard library, so it deploys as a zero-extra-dependency sidecar.
//
// go 1.24 matches the metering module so any product's pod can run the sidecar
// regardless of the product's own toolchain.
module github.com/hanzoai/commerce/metering/proxy

go 1.24

require github.com/hanzoai/commerce/metering v0.0.0

replace github.com/hanzoai/commerce/metering => ../
