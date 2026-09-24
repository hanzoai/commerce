package mpc

import (
	"context"
	"time"
)

// keygenTimeout bounds one threshold keygen on EVERY wire. Keygen needs all
// peers and can take tens of seconds, so the bound is generous; what matters is
// that there is exactly ONE of it. Two transports carrying two timeouts are two
// behaviours, and the one nobody exercises is the one that is wrong.
const keygenTimeout = 120 * time.Second

// Transport carries one MPC operation to the signer fleet and hands back the
// signer's raw response bytes. It knows how bytes travel and nothing else.
//
// Everything that decides what an answer MEANS lives ABOVE this line, in
// GenerateAddress: which chain's address to read out of the reply, that the
// wallet id is kept, that an empty address is a failure rather than a result.
// That placement is the entire point of the seam. Those rules are written once,
// so a correction to one of them cannot be applied to HTTP and forgotten on
// ZAP; a transport that decided any of them would be a second copy of the
// policy, waiting to drift. The transports are consequently interchangeable to
// a caller, and the behavioural tests run the same assertions over both to
// prove it rather than to assert it.
//
// The seam covers keygen and health because those are the operations mpcd
// serves on BOTH wires. The transactional calls (create / approve / refund /
// query) stay on HTTP unconditionally, because mpcd's ZAP surface has no
// opcode for them — see httpTransport.
type Transport interface {
	// Keygen runs one threshold keygen for orgID and returns the signer's raw
	// keygen JSON: the bytes, undecoded and uninterpreted.
	Keygen(ctx context.Context, orgID string) ([]byte, error)

	// Health reports whether the signer fleet is answering. nil means it is.
	Health(ctx context.Context) error
}

// newTransport chooses the wire from configuration, and fails safe: with no
// ZAPAddress set it hands back the HTTP transport this rail has always used, so
// a standalone commerce — a developer's machine, the test suite, a standalone
// deploy — keeps working with no configuration at all and no change in
// behaviour. ZAP is something a deployment opts INTO by naming an address,
// never something it can fall into by accident.
func newTransport(cfg Config, h *httpTransport) Transport {
	if cfg.ZAPAddress == "" {
		return h
	}
	return newZAPTransport(cfg.ZAPAddress)
}
