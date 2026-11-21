// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"net/http"
	"strconv"
)

// Fault is a protocol error and the HTTP status it answers with. A Backend returns Faults for
// what it refuses; the flat Error object of rfc.agentic_checkout.md §3.1 is Body.
type Fault struct {
	Status int
	Body   Error
	// RetryAfter, in seconds, is sent as Retry-After when positive.
	RetryAfter int
}

// Error implements error.
func (f *Fault) Error() string {
	return strconv.Itoa(f.Status) + " " + f.Body.Code + ": " + f.Body.Message
}

func fault(status int, typ, code, msg, param string) *Fault {
	return &Fault{Status: status, Body: Error{Type: typ, Code: code, Message: msg, Param: param}}
}

func missing(param, msg string) *Fault {
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

func noItem(param, id string) *Fault {
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

// NotCancelable refuses to cancel a completed or canceled session (cancelCheckoutSession, 405).
func NotCancelable(msg string) *Fault {
	return fault(http.StatusMethodNotAllowed, TypeInvalidRequest, "not_cancelable", msg, "")
}

// Unavailable answers a failure that may pass; the request is safe to retry.
func Unavailable(msg string) *Fault {
	return fault(http.StatusServiceUnavailable, TypeServiceUnavailable, "service_unavailable", msg, "")
}

func internal() *Fault {
	return fault(http.StatusInternalServerError, TypeProcessingError, "internal_error", "The request could not be processed.", "")
}
