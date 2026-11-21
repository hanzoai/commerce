// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Credential is the payment a client submits for a challenge (section 5.2): the
// challenge it echoes, optionally who pays, and the method's proof.
type Credential struct {
	Challenge Challenge       `json:"challenge"`
	Source    string          `json:"source,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

// errNotPayment says a field value carries another authentication scheme.
var errNotPayment = errors.New("not a Payment credential")

// ParseCredential reads the value of an Authorization or Payment-Authorization field:
// the scheme Payment, white space, and a base64url object without padding that holds
// the echoed challenge, an optional source and a payload object (section 5.2). Unknown
// fields are ignored (section 9.2.1).
func ParseCredential(value string) (*Credential, error) {
	scheme, token, found := strings.Cut(strings.TrimSpace(value), " ")
	if !strings.EqualFold(scheme, Scheme) {
		return nil, errNotPayment
	}
	token = strings.TrimLeft(token, " ")
	if !found || token == "" {
		return nil, errors.New("credential is empty")
	}
	for i := 0; i < len(token); i++ {
		c := token[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return nil, errors.New("credential is not base64url without padding")
		}
	}
	b, err := raw.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("credential is not base64url: %w", err)
	}
	var in struct {
		Challenge *Challenge      `json:"challenge"`
		Source    string          `json:"source"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(b, &in); err != nil {
		return nil, fmt.Errorf("credential is not a JSON object of strings: %w", err)
	}
	if in.Challenge == nil {
		return nil, errors.New("credential has no challenge")
	}
	if p := bytes.TrimSpace(in.Payload); len(p) == 0 || p[0] != '{' {
		return nil, errors.New("credential payload is missing or not an object")
	}
	return &Credential{Challenge: *in.Challenge, Source: in.Source, Payload: in.Payload}, nil
}

// String is the credential as the value of its header field.
func (c Credential) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return Scheme + " " + base64.RawURLEncoding.EncodeToString(b)
}

// Receipt is the server's acknowledgment of a settled payment, the Payment-Receipt
// header (section 5.3). Fields are the additional members a method specification
// defines, such as the Stripe charge draft's externalId.
type Receipt struct {
	Status    string
	Method    string
	Timestamp string
	Reference string
	Fields    map[string]any
}

// MarshalJSON writes the four members of table 5 and then Fields; a field cannot shadow
// one of the four.
func (r Receipt) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	for k, v := range r.Fields {
		m[k] = v
	}
	m["status"], m["method"], m["timestamp"], m["reference"] = r.Status, r.Method, r.Timestamp, r.Reference
	return Canonical(m)
}

// UnmarshalJSON reads the four members and keeps every other in Fields.
func (r *Receipt) UnmarshalJSON(b []byte) error {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	*r = Receipt{}
	for k, v := range m {
		s, isString := v.(string)
		switch {
		case k == "status" && isString:
			r.Status = s
		case k == "method" && isString:
			r.Method = s
		case k == "timestamp" && isString:
			r.Timestamp = s
		case k == "reference" && isString:
			r.Reference = s
		default:
			if r.Fields == nil {
				r.Fields = map[string]any{}
			}
			r.Fields[k] = v
		}
	}
	return nil
}

// String is the receipt as the value of the Payment-Receipt field.
func (r Receipt) String() string {
	b, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// ParseReceipt reads a Payment-Receipt value. The status must be "success", the only
// value a receipt carries (section 5.3.1).
func ParseReceipt(value string) (*Receipt, error) {
	b, err := raw.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("receipt is not base64url without padding: %w", err)
	}
	var r Receipt
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("receipt is not a JSON object: %w", err)
	}
	if r.Status != "success" {
		return nil, fmt.Errorf("receipt status is %q, not success", r.Status)
	}
	if _, err := time.Parse(time.RFC3339, r.Timestamp); err != nil {
		return nil, fmt.Errorf("receipt timestamp is not RFC 3339: %w", err)
	}
	return &r, nil
}
