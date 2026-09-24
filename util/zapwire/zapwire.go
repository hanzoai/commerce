// Package zapwire is commerce's ZAP message framing: the one way this codebase
// puts a request or a reply on a luxfi/zap wire.
//
// The convention was already in use by the ZAP node that serves vector ops and
// SBOM ingest ("the identical [status(1)][reserved(7)][JSON payload] framing").
// It lives here, in a leaf package that imports nothing but luxfi/zap, so that
// a client and a server can share it without a payment rail having to depend on
// the vector/search/storage infrastructure package to learn where a byte goes.
//
// Layout — one 16-byte object:
//
//	offset 0   uint8   status: 0 = ok, non-zero = error
//	offset 1   ..7     reserved
//	offset 8   bytes   payload: the JSON body, or the error text when status≠0
//
// The eight-byte gap is not decoration. A bytes field occupies EIGHT bytes — a
// 4-byte relative pointer followed by a 4-byte length — so consecutive fields
// must be eight apart and the object must reserve through the end of the last
// one. Fields packed one byte apart silently overwrite each other, and a length
// word that runs past the reserved section overwrites the first byte of the
// payload appended after it.
//
// The opcode travels in the message's flags word as opcode<<8, which is where
// zap.Node dispatches from (it handles on Flags()>>8). That leaves the opcode
// eight bits.
package zapwire

import (
	"errors"

	"github.com/luxfi/zap"
)

// Field offsets within the framed object.
const (
	FieldStatus  = 0
	FieldPayload = 8

	// objectSize reserves the whole fixed section: the status byte, the
	// reserved gap, and both words of the payload field.
	objectSize = 16
)

// BuildRequest frames a request carrying payload under opcode.
func BuildRequest(opcode uint16, payload []byte) *zap.Message {
	return frame(opcode<<8, 0, payload)
}

// Response frames a reply. Status 0 means the payload is the result; any other
// status means the payload is the error text.
func Response(status uint8, payload []byte) *zap.Message {
	return frame(0, status, payload)
}

// OK frames a successful reply. payload may be nil.
func OK(payload []byte) *zap.Message { return Response(0, payload) }

// Error frames a failed reply carrying msg as its text.
func Error(msg string) *zap.Message { return Response(1, []byte(msg)) }

// ParseResponse returns a reply's payload, or the error the responder reported.
//
// The returned bytes are COPIED. zap.Object.Bytes hands back a zero-copy view
// into the frame's buffer, which the node may recycle once the caller returns;
// copying here means no caller has to know that.
func ParseResponse(msg *zap.Message) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("zapwire: nil response")
	}
	root := msg.Root()
	payload := root.Bytes(FieldPayload)
	out := make([]byte, len(payload))
	copy(out, payload)

	if root.Uint8(FieldStatus) != 0 {
		if len(out) == 0 {
			return nil, errors.New("zapwire: request failed")
		}
		return nil, errors.New(string(out))
	}
	return out, nil
}

// frame builds the object. Parse cannot fail on a buffer we just wrote — the
// magic, version and size are all written by Builder itself — so there is no
// error here for a caller to handle.
func frame(flags uint16, status uint8, payload []byte) *zap.Message {
	b := zap.NewBuilder(zap.HeaderSize + objectSize + len(payload))
	ob := b.StartObject(objectSize)
	ob.SetUint8(FieldStatus, status)
	if len(payload) > 0 {
		ob.SetBytes(FieldPayload, payload)
	}
	ob.FinishAsRoot()
	msg, _ := zap.Parse(b.FinishWithFlags(flags))
	return msg
}
