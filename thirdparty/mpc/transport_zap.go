package mpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/luxfi/zap"

	"github.com/hanzoai/commerce/util/zapwire"
)

// ZAP opcodes on mpcd's MPC-API surface (lux/mpc pkg/api/zap_server.go).
// A node dispatches on Flags()>>8, so an opcode has eight bits; both of these
// sit well inside it.
const (
	opMPCKeygen uint16 = 0x0060
	opMPCStatus uint16 = 0x0062
)

// zapKeygenReq is the body of an OpMPCKeygen request.
//
// mpcd's ZAP handler reads org_id and wallet_id as separate ZAP text fields
// spaced ONE byte apart (zap_server.go: fieldOrgID 8, fieldWalletID 9), and a
// text field occupies EIGHT bytes — a 4-byte pointer and a 4-byte length. So
// writing the second field destroys the first: measured, setting both and
// reading back yields org_id "" with wallet_id intact. Its handler requires
// both to be non-empty, which means that layout refuses every request any
// client could possibly send. Its reply is framed the same way and fares no
// better: the data field at offset 9 puts its length word one byte past what
// StartObject(16) reserved, overwriting the first byte of the JSON — an 85-byte
// body comes back with '{' replaced by 0x00.
//
// There is therefore no wire-compatible option to preserve, only a correct one
// to choose, and the correct one is already commerce's: a single JSON payload
// in the util/zapwire frame, which is what this rail's own ZAP node has always
// spoken. mpcd has to move to meet it either way.
type zapKeygenReq struct {
	OrgID    string `json:"org_id"`
	WalletID string `json:"wallet_id"`
}

// zapTransport carries keygen and health to mpcd over ZAP.
//
// The address is the whole configuration, and it also selects the medium:
// luxfi/zap's Network() reads a filesystem path (or an "@" abstract name) as a
// unix socket and anything else as host:port, and the dialler and the listener
// share that one rule. "ZAP over TCP" and "ZAP over a unix socket" are therefore
// not two modes here — they are one dialler and two values.
//
// ⚠ FOR THIS HOP THE VALUE IS ALWAYS host:port, and that is a security property
// rather than a deployment accident. mpcd is a THRESHOLD ring: its whole worth
// is that no single host compromise yields a spendable key, which requires the
// nodes to sit on different machines — they do (mpc-node-0/1/2 are on three
// separate workers). A unix socket is same-netns only, so it cannot reach a
// fleet that is deliberately spread, and anything that COULD reach all three
// over a socket would be a host holding the whole ring, which is the thing the
// threshold scheme exists to prevent.
//
// So do not "fix" this to UDS later. The socket path is supported because the
// dialler is one line either way, not because this caller will ever use it. A
// co-resident socket is right for a sidecar; it is wrong for a quorum.
type zapTransport struct {
	addr   string
	nodeID string

	mu     sync.Mutex
	node   *zap.Node
	peerID string
}

func newZAPTransport(addr string) *zapTransport {
	return &zapTransport{addr: addr, nodeID: "commerce-mpc-" + randomHex(8)}
}

// conn returns a connected node, dialling on first use and re-dialling once a
// peer has gone away. Connecting lazily keeps a signer that is down at boot
// from being a boot failure: the rail reports it on the call that needed it,
// exactly as the HTTP wire does.
func (t *zapTransport) conn() (*zap.Node, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.node == nil {
		// No Start(): a client has nothing to serve, and ConnectDirectID runs
		// the receive loop for the connection it opens. Starting would bind a
		// listening socket for no reason.
		//
		// The node id carries randomness because a server keys its connections
		// by it — two commerce replicas dialling one signer under the same id
		// would collide in that map.
		t.node = zap.NewNode(zap.NodeConfig{
			NodeID:      t.nodeID,
			ServiceType: "_mpc._tcp",
			NoDiscovery: true,
		})
	}
	if t.peerID != "" && t.connected(t.peerID) {
		return t.node, t.peerID, nil
	}

	peerID, err := t.node.ConnectDirectID(t.addr)
	if err != nil {
		return nil, "", fmt.Errorf("mpc: zap connect to %s: %w", t.addr, err)
	}
	t.peerID = peerID
	return t.node, peerID, nil
}

func (t *zapTransport) connected(peerID string) bool {
	for _, p := range t.node.Peers() {
		if p == peerID {
			return true
		}
	}
	return false
}

// forget drops the cached peer so the next call re-dials. A dead connection is
// indistinguishable here from a slow one, and the cheap repair is to redial
// rather than to diagnose.
func (t *zapTransport) forget() {
	t.mu.Lock()
	t.peerID = ""
	t.mu.Unlock()
}

// call carries one opcode with a JSON body and returns the reply's payload.
func (t *zapTransport) call(ctx context.Context, op uint16, body []byte) ([]byte, error) {
	node, peerID, err := t.conn()
	if err != nil {
		return nil, err
	}

	resp, err := node.Call(ctx, peerID, zapwire.BuildRequest(op, body))
	if err != nil {
		t.forget()
		return nil, fmt.Errorf("mpc: zap op %#04x to %s failed: %w", op, t.addr, err)
	}

	payload, err := zapwire.ParseResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("mpc: zap op %#04x to %s: %w", op, t.addr, err)
	}
	return payload, nil
}

// Keygen runs one threshold keygen and returns the signer's raw keygen JSON.
//
// The wallet id is minted HERE, which is the one place the two wires genuinely
// differ, and it is mpcd that forces it: its HTTP handler mints an id when the
// request omits one, its ZAP handler refuses a request that does not carry one.
// The property the rail depends on is that every keygen gets a FRESH,
// unpredictable id — a replayed id re-keys an existing wallet and silently
// moves an address that funds may already be in flight to — and minting from
// crypto/rand preserves it, never from the customer, the clock, or anything
// else a caller could repeat. Which side of the wire produces the id is not
// something the rail can observe; that it is never reused is.
func (t *zapTransport) Keygen(ctx context.Context, orgID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, keygenTimeout)
	defer cancel()

	body, err := json.Marshal(zapKeygenReq{
		OrgID: orgID,
		// 32 hex characters, the shape mpcd's own minting emits.
		WalletID: randomHex(16),
	})
	if err != nil {
		return nil, fmt.Errorf("mpc: marshal keygen request: %w", err)
	}
	return t.call(ctx, opMPCKeygen, body)
}

// Health asks the fleet for its cluster status. Any answer at all means the
// signer is reachable and serving; what it says is the dashboard's business.
func (t *zapTransport) Health(ctx context.Context) error {
	_, err := t.call(ctx, opMPCStatus, nil)
	return err
}

// randomHex returns 2n hex characters of cryptographic randomness. crypto/rand
// Read cannot fail on any supported platform — it panics internally rather than
// returning a short read — so there is no error to handle here and no weaker
// fallback to be tempted by.
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Ensure zapTransport satisfies the seam.
var _ Transport = (*zapTransport)(nil)
