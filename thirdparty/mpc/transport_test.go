package mpc

import "testing"

// An unset ZAP address must leave the rail on HTTP. This is the fail-safe that
// lets a standalone commerce — a developer's machine, this test binary, a
// standalone deploy — keep working with no configuration at all: ZAP is opted
// into by naming an address, never fallen into.
func TestTransport_UnsetAddressStaysOnHTTP(t *testing.T) {
	mp := NewProcessor(Config{
		KMSEndpoint: "http://kms.example",
		MPCEndpoint: "http://mpc.example",
	})

	if _, ok := mp.transport.(*httpTransport); !ok {
		t.Fatalf("transport is %T with no ZAP address configured, want *httpTransport", mp.transport)
	}
	// Same object, not a second one: there is one piece of HTTP plumbing, so
	// the keygen wire and the transactional wire cannot be fixed separately.
	if mp.transport != Transport(mp.http) {
		t.Error("the HTTP transport was built twice")
	}
}

// Naming an address selects ZAP. Both spellings select the SAME transport —
// the address is passed to luxfi/zap, which derives tcp from a host:port and
// unix from a path, so there is no second setting that could disagree with it.
func TestTransport_AddressSelectsZAP(t *testing.T) {
	for _, addr := range []string{
		"mpc-node.hanzo-mpc.svc.cluster.local:9805", // TCP
		"/run/mpc/zap.sock",                         // unix socket
	} {
		t.Run(addr, func(t *testing.T) {
			mp := NewProcessor(Config{
				KMSEndpoint: "http://kms.example",
				MPCEndpoint: "http://mpc.example",
				ZAPAddress:  addr,
			})

			zt, ok := mp.transport.(*zapTransport)
			if !ok {
				t.Fatalf("transport is %T with ZAPAddress %q, want *zapTransport", mp.transport, addr)
			}
			if zt.addr != addr {
				t.Errorf("transport dials %q, want %q", zt.addr, addr)
			}
			// The transactional calls have no ZAP opcode on mpcd, so they must
			// still hold their HTTP endpoint even when keygen went to ZAP.
			if mp.http == nil || mp.http.endpoint != "http://mpc.example" {
				t.Errorf("HTTP endpoint lost when ZAP was selected: %+v", mp.http)
			}
		})
	}
}

// Selecting ZAP must not dial. A signer that is down when commerce boots is a
// failed keygen later, not a process that will not start.
func TestTransport_SelectingZAPDoesNotDial(t *testing.T) {
	mp := NewProcessor(Config{
		KMSEndpoint: "http://kms.example",
		MPCEndpoint: "http://mpc.example",
		// Nothing listens here, and constructing must not care.
		ZAPAddress: "127.0.0.1:1",
	})

	zt, ok := mp.transport.(*zapTransport)
	if !ok {
		t.Fatalf("transport is %T, want *zapTransport", mp.transport)
	}
	if zt.node != nil || zt.peerID != "" {
		t.Error("construction opened a connection to the signer")
	}
}

// The policy's mpc account is what selects the wire in a running commerce, and a setting
// that never reaches the processor is a feature that does nothing. This walks the real
// path — DefaultConfig, NewProcessor, Configure — because that is the path the runtime
// takes, and it reads nothing from the environment.
func TestConfigure_TheAccountSelectsTheWireAndTheSecrets(t *testing.T) {
	t.Run("unset stays on HTTP and off", func(t *testing.T) {
		mp := NewProcessor(DefaultConfig())
		if _, ok := mp.transport.(*httpTransport); !ok {
			t.Fatalf("transport is %T before any account, want *httpTransport", mp.transport)
		}
		if mp.configured() {
			t.Error("the rail is configured before the policy's mpc account names an endpoint")
		}
		mp.Configure(Settings{Endpoint: "https://mpc.example", KMSEndpoint: "https://kms.example", WebhookSecret: "whsec"})
		if !mp.configured() || mp.kmsEndpoint != "https://kms.example" || mp.secret() != "whsec" || !mp.ReceivesWebhooks() {
			t.Errorf("rail = configured %v kms %q webhooks %v; want the account's settings", mp.configured(), mp.kmsEndpoint, mp.ReceivesWebhooks())
		}
		mp.Configure(Settings{})
		if mp.configured() || mp.ReceivesWebhooks() {
			t.Error("an empty account left the rail on")
		}
	})

	for _, addr := range []string{
		"mpc-node-headless.hanzo-mpc.svc.cluster.local:9805",
		"/run/mpc/zap.sock",
	} {
		t.Run("zap="+addr, func(t *testing.T) {
			mp := NewProcessor(DefaultConfig())
			mp.Configure(Settings{Endpoint: "https://mpc.example", ZAPAddress: addr})
			zt, ok := mp.wire().(*zapTransport)
			if !ok {
				t.Fatalf("transport is %T with zap=%q, want *zapTransport", mp.wire(), addr)
			}
			if zt.addr != addr {
				t.Errorf("transport dials %q, want %q", zt.addr, addr)
			}
			// And back to HTTP when the account drops it.
			mp.Configure(Settings{Endpoint: "https://mpc.example"})
			if _, ok := mp.wire().(*httpTransport); !ok {
				t.Errorf("transport is %T after the zap address was removed, want *httpTransport", mp.wire())
			}
		})
	}

	// Surrounding whitespace is a configuration accident, not an address.
	t.Run("whitespace is not an address", func(t *testing.T) {
		mp := NewProcessor(DefaultConfig())
		mp.Configure(Settings{Endpoint: "https://mpc.example", ZAPAddress: "   "})
		if _, ok := mp.wire().(*httpTransport); !ok {
			t.Fatalf("transport is %T for a blank zap address, want *httpTransport", mp.wire())
		}
	})
}
