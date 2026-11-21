// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Code is a problem type of the error table (section 8.2): the last segment of its URI.
type Code string

// The problem types; the last four are in the working copy, not published draft-01.
const (
	Required     Code = "payment-required"
	Insufficient Code = "payment-insufficient"
	Expired      Code = "payment-expired"
	Failed       Code = "verification-failed"
	Unsupported  Code = "method-unsupported"
	Malformed    Code = "malformed-credential"
	Invalid      Code = "invalid-challenge"
	Bad          Code = "bad-request"
	Payload      Code = "invalid-payload"
	Internal     Code = "internal-payment-error"
	Action       Code = "payment-action-required"
)

// base is the base URI of the problem types (section 8.1).
const base = "https://paymentauth.org/problems/"

// status is the HTTP status of each problem type (section 8.2).
var status = map[Code]int{
	Required: 402, Insufficient: 402, Expired: 402, Failed: 402, Unsupported: 400,
	Malformed: 402, Invalid: 402, Bad: 400, Payload: 402, Internal: 500, Action: 402,
}

// title is the problem's title: the two the draft's examples spell out (section 8.1,
// appendix B.5), else the code in words.
func title(c Code) string {
	switch c {
	case Required:
		return "Payment Required"
	case Failed:
		return "Payment Verification Failed"
	}
	words := strings.Fields(strings.ReplaceAll(string(c), "-", " "))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// Problem is an RFC 9457 problem details object (section 8.1). A Method returns one as
// an error to say the payment was refused and nothing charged; any other error means the
// outcome is not known.
type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
	// Challenge is the challengeId extension member (appendix B.1).
	Challenge string `json:"challengeId,omitempty"`
}

// Refuse is the problem of a type, with an explanation.
func Refuse(c Code, detail string) *Problem {
	s, ok := status[c]
	if !ok {
		c, s = Internal, 500
	}
	return &Problem{Type: base + string(c), Title: title(c), Status: s, Detail: detail}
}

func (p *Problem) Error() string {
	return p.Title + ": " + p.Detail
}

// write sends the problem as application/problem+json.
func (p *Problem) write(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
