// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package woocommerce implements Hanzo for WooCommerce (/v1/source/woocommerce/*):
// per-shop secret connect, Client::sign HMAC-SHA256 verification, REST v3 batch
// ingest (products + embedded variations, orders + guest consent + refunds,
// customers), GDPR export & erase (including guest:<order_id>), and bidirectional
// outbound push/pull sync.
package woocommerce

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MaxClockSkew = 5 * time.Minute

var idTokenRe = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// MintConnectionSecret generates a 32-byte (64 lowercase hex chars) per-shop HMAC secret,
// matching Client::connect()'s /^[0-9a-f]{64}$/ validation in hanzo/woocommerce.
func MintConnectionSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Sign computes the exact v1 hex HMAC-SHA256 signature implemented by Hanzo\Client::sign
// in hanzo/woocommerce/includes/class-client.php:
//
//	hash_hmac('sha256', t + "\n" + shop + "\n" + topic + "\n" + delivery + "\n" + body, secret)
func Sign(secret string, t int64, shop, topic, delivery string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(t, 10)))
	mac.Write([]byte("\n"))
	mac.Write([]byte(shop))
	mac.Write([]byte("\n"))
	mac.Write([]byte(topic))
	mac.Write([]byte("\n"))
	mac.Write([]byte(delivery))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// FormatSignatureHeader formats X-Hanzo-Signature: t=<unix>,v1=<hex>.
func FormatSignatureHeader(secret string, t int64, shop, topic, delivery string, body []byte) string {
	return "t=" + strconv.FormatInt(t, 10) + ",v1=" + Sign(secret, t, shop, topic, delivery, body)
}

// VerifySignatureHeader validates X-Hanzo-Signature against secret, shop, topic,
// delivery, raw body, and timestamp freshness window.
func VerifySignatureHeader(secret, header, shop, topic, delivery string, body []byte, now time.Time) bool {
	if secret == "" || header == "" || !idTokenRe.MatchString(shop) || topic == "" || !idTokenRe.MatchString(delivery) {
		return false
	}
	var tsStr, v1Hex string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			tsStr = v
		case "v1":
			v1Hex = v
		}
	}
	if tsStr == "" || len(v1Hex) != 64 {
		return false
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil || ts <= 0 {
		return false
	}
	if !now.IsZero() {
		diff := now.Sub(time.Unix(ts, 0))
		if diff < -MaxClockSkew || diff > MaxClockSkew {
			return false
		}
	}
	got, err := hex.DecodeString(v1Hex)
	if err != nil || len(got) != sha256.Size {
		return false
	}
	wantHex := Sign(secret, ts, shop, topic, delivery, body)
	want, _ := hex.DecodeString(wantHex)
	return hmac.Equal(got, want)
}

// VerifyWCWebhookSignature checks native WooCommerce X-WC-Webhook-Signature (base64 HMAC-SHA256).
func VerifyWCWebhookSignature(secret, headerB64 string, body []byte) bool {
	if secret == "" || strings.TrimSpace(headerB64) == "" {
		return false
	}
	got, err := base64.StdEncoding.DecodeString(strings.TrimSpace(headerB64))
	if err != nil || len(got) != sha256.Size {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}
