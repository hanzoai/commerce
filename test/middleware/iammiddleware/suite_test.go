// Copyright (c) 2026-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.
//
// Suite shim. The previous JWT/JWKS test rig (~188 LOC of RSA keys +
// httptest JWKS endpoint + claim helpers) is no longer applicable
// since the trust boundary moved to hanzoai/gateway. Tests in this
// package now use stdlib net/http + httptest only.

package test
