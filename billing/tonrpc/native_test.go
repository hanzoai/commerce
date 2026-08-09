package tonrpc

import (
	"strings"
	"testing"
)

// A watched deposit account, and a sender.
const (
	acct = "UQB6b9lZVanb-8w_sUn4NZ8clDs5dw9QghJxYeT87GTYRHye"
	from = "UQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIsw"
)

// nativeTx builds a transaction whose inbound message is what the test is about.
// Everything not named is the "it worked" shape, so each test states only its
// own difference.
func nativeTx(mutate func(*transaction)) *transaction {
	t := &transaction{
		Account:      acct,
		Hash:         "3F3ynGXMHDpVSHiJhi/PYnHXOBs4FXMxsRs5DwjOSbA=",
		Lt:           "12345",
		McBlockSeqno: 100,
	}
	t.InMsg = &struct {
		Source         string `json:"source"`
		Destination    string `json:"destination"`
		Opcode         string `json:"opcode"`
		Bounced        bool   `json:"bounced"`
		Value          string `json:"value"`
		MessageContent *struct {
			Decoded map[string]any `json:"decoded"`
		} `json:"message_content"`
	}{
		Source:      from,
		Destination: acct,
		Value:       "2500000000", // 2.5 TON in nanotons
	}
	t.Description.ComputePh = &struct {
		Skipped  bool `json:"skipped"`
		Success  bool `json:"success"`
		ExitCode int  `json:"exit_code"`
	}{Success: true}
	if mutate != nil {
		mutate(t)
	}
	return t
}

func nativeParse(t *testing.T, x *transaction) (units string, credited bool) {
	t.Helper()
	a, err := ParseAddress(acct)
	if err != nil {
		t.Fatalf("ParseAddress: %v", err)
	}
	got, ok, err := nativeReceipt(x, a, acct)
	if err != nil {
		t.Fatalf("nativeReceipt: %v", err)
	}
	if !ok {
		return "", false
	}
	return got.Units.String(), true
}

func TestAPlainTransferIsADeposit(t *testing.T) {
	units, ok := nativeParse(t, nativeTx(nil))
	if !ok {
		t.Fatal("a plain inbound transfer credited nothing")
	}
	if units != "2500000000" {
		t.Errorf("units = %s, want 2500000000 (2.5 TON)", units)
	}
}

// THE ONE THAT MATTERS. Every inbound message carries a value, including the gas
// attached to a contract call. A jetton transfer notification lands on this very
// account with a little TON attached to pay for it — crediting that would turn
// every jetton transfer into a small phantom TON deposit, on every deposit
// address, forever.
func TestGasAttachedToAContractCallIsNotADeposit(t *testing.T) {
	for _, op := range []string{
		"0x7362d09c", // jetton transfer_notification
		"0x178d4519", // jetton internal_transfer
		"0xd53276db", // excesses
		"0x2c76b973", // an arbitrary contract op seen live on mainnet
	} {
		if _, ok := nativeParse(t, nativeTx(func(x *transaction) { x.InMsg.Opcode = op })); ok {
			t.Errorf("opcode %s was credited as a native deposit — that value is gas", op)
		}
	}
}

// A plain transfer carrying a TEXT COMMENT uses the explicit zero opcode, and it
// is how most people send TON with a memo. Refusing it would drop real deposits.
func TestAZeroOpcodeIsStillAPlainTransfer(t *testing.T) {
	for _, op := range []string{"0x00000000", "0x0", "0", ""} {
		units, ok := nativeParse(t, nativeTx(func(x *transaction) { x.InMsg.Opcode = op }))
		if !ok {
			t.Errorf("opcode %q credited nothing — a commented transfer is a deposit", op)
			continue
		}
		if units != "2500000000" {
			t.Errorf("opcode %q credited %s", op, units)
		}
	}
}

func TestABounceIsNotADeposit(t *testing.T) {
	// Money coming back from something WE sent is not somebody paying us.
	if _, ok := nativeParse(t, nativeTx(func(x *transaction) { x.InMsg.Bounced = true })); ok {
		t.Error("a bounced message was credited")
	}
}

func TestAFailedTransactionCreditsNothing(t *testing.T) {
	for name, mutate := range map[string]func(*transaction){
		"aborted":        func(x *transaction) { x.Description.Aborted = true },
		"compute failed": func(x *transaction) { x.Description.ComputePh.Success = false },
		"compute skipped": func(x *transaction) {
			x.Description.ComputePh.Skipped = true
		},
		"nonzero exit": func(x *transaction) { x.Description.ComputePh.ExitCode = 1 },
		"action failed": func(x *transaction) {
			x.Description.Action = &struct {
				Success    bool `json:"success"`
				ResultCode int  `json:"result_code"`
			}{Success: false}
		},
	} {
		if _, ok := nativeParse(t, nativeTx(mutate)); ok {
			t.Errorf("%s: credited despite failing on chain", name)
		}
	}
}

// An inbound message with no source is an EXTERNAL message — the account's owner
// driving their own wallet with a key, not value arriving from another account.
func TestAnExternalMessageIsNotADeposit(t *testing.T) {
	if _, ok := nativeParse(t, nativeTx(func(x *transaction) { x.InMsg.Source = "" })); ok {
		t.Error("an external message was credited as a deposit")
	}
}

func TestZeroValueCreditsNothing(t *testing.T) {
	if _, ok := nativeParse(t, nativeTx(func(x *transaction) { x.InMsg.Value = "0" })); ok {
		t.Error("a zero-value message was credited")
	}
}

// A missing value is a SHAPE this code does not understand, not a zero. Guessing
// zero would silently drop a real deposit.
func TestAMissingValueIsAnError(t *testing.T) {
	a, _ := ParseAddress(acct)
	_, _, err := nativeReceipt(nativeTx(func(x *transaction) { x.InMsg.Value = "" }), a, acct)
	if err == nil {
		t.Fatal("a plain message with no value was accepted")
	}
	if !strings.Contains(err.Error(), "no value") {
		t.Errorf("error does not say the value was missing: %v", err)
	}
}

func TestNoInboundMessageCreditsNothing(t *testing.T) {
	if _, ok := nativeParse(t, nativeTx(func(x *transaction) { x.InMsg = nil })); ok {
		t.Error("an outbound-only transaction was credited")
	}
}

func TestIsZeroOpcode(t *testing.T) {
	for _, yes := range []string{"", "0", "0x0", "0x00000000", "0X0000"} {
		if !isZeroOpcode(yes) {
			t.Errorf("isZeroOpcode(%q) = false, want true", yes)
		}
	}
	for _, no := range []string{"0x7362d09c", "0x1", "0x00000001", "abc"} {
		if isZeroOpcode(no) {
			t.Errorf("isZeroOpcode(%q) = true, want false", no)
		}
	}
}
