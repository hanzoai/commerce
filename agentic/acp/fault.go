// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"net/http"
	"strconv"
)

// Fault is a protocol error with the HTTP status it answers with. A Backend returns Faults
// for what it refuses; the service turns them into the flat Error object of the spec
// (rfc.agentic_checkout.md §3.1) without changing them.
type Fault struct {
	Status int
	Body   Error
	// RetryAfter, in seconds, is sent as Retry-After when positive (idempotency_in_flight).
	RetryAfter int
}

// Error implements error.
func (f *Fault) Error() string {
	return strconv.Itoa(f.Status) + " " + f.Body.Code + ": " + f.Body.Message
}

func fault(status int, typ, code, msg, param string) *Fault {
	return &Fault{Status: status, Body: Error{Type: typ, Code: code, Message: msg, Param: param}}
}

// Missing refuses a request that lacks a required field.
func Missing(param, msg string) *Fault {
	return fault(http.StatusBadRequest, TypeInvalidRequest, "missing_required_field", msg, param)
}

// Invalid refuses a request with a malformed or unacceptable value.
func Invalid(param, msg string) *Fault {
	return fault(http.StatusBadRequest, TypeInvalidRequest, "invalid", msg, param)
}

// Unsupported refuses a well-formed request this seller cannot honour.
func Unsupported(param, msg string) *Fault {
	return fault(http.StatusBadRequest, TypeInvalidRequest, "unsupported", msg, param)
}

// NoItem refuses an item id the seller does not sell.
func NoItem(param, id string) *Fault {
	return fault(http.StatusBadRequest, TypeInvalidRequest, "invalid_item_id", "The item ID '"+id+"' does not exist.", param)
}

// NotFound answers a session that does not exist for the caller.
func NotFound(msg string) *Fault {
	return fault(http.StatusNotFound, TypeInvalidRequest, "not_found", msg, "")
}

// Conflict refuses a change the session's state does not allow.
func Conflict(msg string) *Fault {
	return fault(http.StatusConflict, TypeInvalidRequest, "conflict", msg, "")
}

// NotCancelable refuses to cancel a session that is already completed or canceled
// (openapi cancelCheckoutSession, 405).
func NotCancelable(msg string) *Fault {
	return fault(http.StatusMethodNotAllowed, TypeInvalidRequest, "not_cancelable", msg, "")
}

// Unavailable answers a failure that may pass: the request is safe to retry.
func Unavailable(msg string) *Fault {
	return fault(http.StatusServiceUnavailable, TypeServiceUnavailable, "service_unavailable", msg, "")
}

// Internal answers an unexpected failure of the seller.
func Internal() *Fault {
	return fault(http.StatusInternalServerError, TypeProcessingError, "internal_error", "The request could not be processed.", "")
}
