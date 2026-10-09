// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package mail is commerce's outbound plain-text mail, delivered by the HOST.
//
// One Sender, installed once at embed time (EmbedConfig.Mail), the same way the
// credit ledger and the secret reader are: commerce asks its host rather than
// holding a mail provider credential of its own. In the cloud binary the host's
// sender is its notify rail, whose provider credentials live in KMS.
//
// nil is a supported state: a standalone commerce with no host sends nothing,
// and Send says so with ErrNoSender instead of pretending.
package mail

import (
	"context"
	"errors"
	"sync"
)

// Sender delivers one plain-text message to every address in to.
type Sender interface {
	Send(ctx context.Context, to []string, subject, body string) error
}

// ErrNoSender is Send without a host sender installed.
var ErrNoSender = errors.New("mail: the host installed no sender")

var (
	mu sync.RWMutex
	s  Sender
)

// Set installs the host's sender; nil removes it. Called once, before routes
// register.
func Set(x Sender) {
	mu.Lock()
	defer mu.Unlock()
	s = x
}

// Send delivers through the installed sender.
func Send(ctx context.Context, to []string, subject, body string) error {
	mu.RLock()
	x := s
	mu.RUnlock()
	if x == nil {
		return ErrNoSender
	}
	return x.Send(ctx, to, subject, body)
}
