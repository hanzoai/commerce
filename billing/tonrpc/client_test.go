package tonrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Reader-level tests over the REAL wire shapes of the TON Index v3 API.
//
// Two of them exist because of measured behaviour of that API rather than
// anything about TON: several of its query parameters are SILENTLY IGNORED
// rather than rejected, so a filter can read as success while filtering
// nothing. This client verifies every response against what it asked for, and
// these prove the verification works.

const (
	ownerRaw    = "0:852443F8599FE6A5DA34FE43049AC4E0BEB3071BB2BFB56635EA9421287C283A"
	walletRaw   = "0:F4B4D8FC356F55852644F232B9150F57888AF53C5AB82DB625658D43E51876BD"
	strangerRaw = "0:AB402915490B6803EBE7A30DFF0CA71880843050C7E4ACDF116F8C5C595F0110"
	txHashB64   = "1YabVkukw02sgxSTXJDesmBJnzQn0qqd1yBw1KNosgw="
	txHashHex   = "d5869b564ba4c34dac8314935c90deb260499f3427d2aa9dd72070d4a368b20c"
)

// serve stands up a fake TON Index that answers each path from a table.
func serve(t *testing.T, answers map[string]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := answers[r.URL.Path]
		if !ok {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	master, err := ParseAddress(usdtMasterEQ)
	if err != nil {
		t.Fatal(err)
	}
	return NewClient(srv.URL, master)
}

const walletsAnswer = `{"jetton_wallets":[{"address":"` + walletRaw + `","owner":"` + ownerRaw + `","jetton":"` + usdtMasterRaw + `","balance":"1219647"}]}`

// tx renders one transaction on the jetton wallet.
func tx(opcode, dest, amount, hash string, mcSeqno, lt string, aborted, computeOK bool) string {
	ab, co := "false", "true"
	if aborted {
		ab = "true"
	}
	if !computeOK {
		co = "false"
	}
	return `{"account":"` + walletRaw + `","hash":"` + hash + `","lt":"` + lt + `","mc_block_seqno":` + mcSeqno + `,` +
		`"in_msg":{"source":"0:SRC","destination":"` + dest + `","opcode":"` + opcode + `","bounced":false,` +
		`"message_content":{"decoded":{"@type":"jetton_internal_transfer","query_id":"1",` +
		`"amount":{"@type":"var_uint","len":"3","value":"` + amount + `"}}}},` +
		`"description":{"aborted":` + ab + `,"compute_ph":{"skipped":false,"success":` + co + `,"exit_code":0},` +
		`"action":{"success":true,"result_code":0}}}`
}

// THE test for this client. TON Index records a jetton transfer against the
// SENDER's wallet transaction (opcode 0x0f8a7ea5, `transfer`), which means the
// tokens were SENT — not that they arrived. A transfer can still fail at the
// destination and bounce back. This client credits the RECEIVING side only
// (opcode 0x178d4519, `internal_transfer`), and that distinction is the
// difference between crediting money and crediting an intention.
func TestReceipts_CreditsTheReceivingSideOnly(t *testing.T) {
	c := serve(t, map[string]string{
		"/jetton/wallets": walletsAnswer,
		"/transactions": `{"transactions":[` +
			tx("0x178d4519", walletRaw, "2250000", txHashB64, "900", "95103197000003", false, true) +
			`]}`,
	})
	got, err := c.TransfersTo(context.Background(), []string{ownerRaw}, 800, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("read %d transfers, want 1", len(got))
	}
	if got[0].Units.String() != "2250000" {
		t.Fatalf("amount = %s, want 2250000", got[0].Units)
	}
	if got[0].To != ownerRaw {
		t.Fatalf("addressed to %q, want the OWNER string the caller passed (%q), not the jetton wallet", got[0].To, ownerRaw)
	}
	if got[0].Block != 900 {
		t.Fatalf("block = %d, want the masterchain seqno 900", got[0].Block)
	}
	if got[0].TxHash != txHashHex {
		t.Fatalf("transaction id = %q, want canonical lowercase hex %q", got[0].TxHash, txHashHex)
	}
	if got[0].EventIndex != 0 {
		t.Fatalf("event index = %d; a TON transaction has at most one inbound message", got[0].EventIndex)
	}
	if got[0].Tag != "" {
		t.Fatalf("TON carries no routing tag, but one came back: %q", got[0].Tag)
	}
}

// Everything that did not actually move tokens into our wallet reads as no
// deposit.
func TestReceipts_IgnoresWhatDidNotArrive(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{
			// The sender-side record. Crediting this is crediting a transfer
			// that may never have landed.
			"the sender-side transfer message",
			tx("0x0f8a7ea5", walletRaw, "2250000", txHashB64, "900", "95103197000003", false, true),
		},
		{
			// The wallet contract validates its sender before crediting, so a
			// forged credit FAILS on chain. Reading the failure is what carries
			// that consensus-enforced check into this process.
			"a transaction that aborted",
			tx("0x178d4519", walletRaw, "2250000", txHashB64, "900", "95103197000003", true, true),
		},
		{
			"a transaction whose compute phase failed",
			tx("0x178d4519", walletRaw, "2250000", txHashB64, "900", "95103197000003", false, false),
		},
		{
			"a plain TON transfer carrying no jettons",
			tx("0x00000000", walletRaw, "2250000", txHashB64, "900", "95103197000003", false, true),
		},
		{
			// At the very tip, indexed but not yet placed in a masterchain
			// block. It must be SKIPPED without stopping the walk, and credited
			// by a later pass.
			"a transaction not yet in a masterchain block",
			tx("0x178d4519", walletRaw, "2250000", txHashB64, "0", "95103197000003", false, true),
		},
		{
			"a transaction newer than the window",
			tx("0x178d4519", walletRaw, "2250000", txHashB64, "5000", "95103197000003", false, true),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := serve(t, map[string]string{
				"/jetton/wallets": walletsAnswer,
				"/transactions":   `{"transactions":[` + tc.body + `]}`,
			})
			got, err := c.TransfersTo(context.Background(), []string{ownerRaw}, 800, 1000)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("read %d transfers: %+v", len(got), got)
			}
		})
	}
}

// This API silently ignores some filter parameters rather than rejecting them,
// so a response about the WRONG account arrives looking exactly like success.
// Trusting it would credit one customer's deposit to another.
func TestReceipts_RefusesAnAnswerAboutADifferentAccount(t *testing.T) {
	c := serve(t, map[string]string{
		"/jetton/wallets": walletsAnswer,
		"/transactions": `{"transactions":[` +
			strings.Replace(tx("0x178d4519", walletRaw, "999", txHashB64, "900", "95103197000003", false, true),
				`"account":"`+walletRaw+`"`, `"account":"`+strangerRaw+`"`, 1) +
			`]}`,
	})
	if _, err := c.TransfersTo(context.Background(), []string{ownerRaw}, 800, 1000); err == nil {
		t.Fatal("credited transactions belonging to another account")
	} else if !strings.Contains(err.Error(), "ignored the account filter") {
		t.Fatalf("error %q does not name the problem", err)
	}
}

// Same hazard on the owner→wallet resolution: /jetton/wallets ignores an
// unrecognised jetton filter, so the answer must be checked against what was
// asked for.
func TestJettonWallet_RefusesAWalletOfAnotherJettonOrOwner(t *testing.T) {
	c := serve(t, map[string]string{
		"/jetton/wallets": `{"jetton_wallets":[{"address":"` + walletRaw + `","owner":"` + ownerRaw + `","jetton":"0:0000000000000000000000000000000000000000000000000000000000000001"}]}`,
	})
	got, err := c.TransfersTo(context.Background(), []string{ownerRaw}, 800, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("read %d transfers from a wallet of a DIFFERENT jetton", len(got))
	}
}

// An owner with no jetton wallet has simply never been paid: on TON the wallet
// contract is deployed by the first transfer into it, exactly as a Solana ATA
// is. Nothing to read, and emphatically not an error.
func TestJettonWallet_AnUnfundedOwnerIsNotAnError(t *testing.T) {
	c := serve(t, map[string]string{"/jetton/wallets": `{"jetton_wallets":[]}`})
	got, err := c.TransfersTo(context.Background(), []string{ownerRaw}, 800, 1000)
	if err != nil {
		t.Fatalf("an owner who has never received anything was an error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("read %d transfers for an owner with no wallet", len(got))
	}
}

// Two watched addresses that decode to ONE account — a case that cannot arise
// on Solana, where base58 is canonical, but does on TON, where an account has
// several correct spellings. "Which of these intents owns it?" has no safe
// answer, so the pass stops.
func TestTransfersTo_RefusesTwoSpellingsOfOneAccount(t *testing.T) {
	c := serve(t, map[string]string{})
	_, err := c.TransfersTo(context.Background(), []string{usdtMasterEQ, usdtMasterUQ}, 800, 1000)
	if err == nil {
		t.Fatal("accepted the same account written two ways as two watched addresses")
	}
	if !strings.Contains(err.Error(), "same account") {
		t.Fatalf("error %q does not name the collision", err)
	}
}

// Decimals come off the master's ON-CHAIN content dictionary. A jetton that
// publishes them only in the off-chain JSON its URI points at is REFUSED: the
// scale of a credit must not depend on a web server the issuer controls.
func TestDecimalsAndSymbol_ReadOnlyOnChainContent(t *testing.T) {
	onChain := serve(t, map[string]string{
		"/jetton/masters": `{"jetton_masters":[{"address":"` + usdtMasterRaw + `","jetton_content":{"decimals":"6","symbol":"TESTUSD","uri":"https://example.test/t.json"}}]}`,
	})
	if d, err := onChain.Decimals(context.Background()); err != nil || d != 6 {
		t.Fatalf("Decimals = %d, %v", d, err)
	}
	if s, err := onChain.Symbol(context.Background()); err != nil || s != "TESTUSD" {
		t.Fatalf("Symbol = %q, %v", s, err)
	}

	// USDT on TON, exactly as mainnet publishes it: decimals and a URI on
	// chain, the ticker only in the off-chain document — where it is "USD₮".
	offChain := serve(t, map[string]string{
		"/jetton/masters": `{"jetton_masters":[{"address":"` + usdtMasterRaw + `","jetton_content":{"decimals":"6","uri":"https://tether.to/usdt-ton.json"}}],` +
			`"metadata":{"` + usdtMasterRaw + `":{"token_info":[{"symbol":"USD₮"}]}}}`,
	})
	if d, err := offChain.Decimals(context.Background()); err != nil || d != 6 {
		t.Fatalf("Decimals = %d, %v — USDT does publish these on chain", d, err)
	}
	sym, err := offChain.Symbol(context.Background())
	if err == nil {
		t.Fatalf("Symbol = %q — a jetton that does not say ON CHAIN what it is must be refused", sym)
	}
	if !strings.Contains(err.Error(), "USD₮") {
		t.Fatalf("the refusal does not tell the operator what the off-chain metadata claims: %v", err)
	}

	// And a jetton with NO on-chain decimals at all is refused rather than
	// scaled from an issuer's web server.
	noDecimals := serve(t, map[string]string{
		"/jetton/masters": `{"jetton_masters":[{"address":"` + usdtMasterRaw + `","jetton_content":{"uri":"https://example.test/t.json"}}]}`,
	})
	if d, err := noDecimals.Decimals(context.Background()); err == nil {
		t.Fatalf("Decimals = %d for a jetton that publishes none on chain", d)
	}
}

// The masterchain seqno is the scan window, and it must come from the
// masterchain.
func TestBlockNumber(t *testing.T) {
	c := serve(t, map[string]string{
		"/masterchainInfo": `{"last":{"seqno":84467002,"workchain":-1}}`,
	})
	head, err := c.BlockNumber(context.Background())
	if err != nil || head != 84467002 {
		t.Fatalf("BlockNumber = %d, %v", head, err)
	}

	wrong := serve(t, map[string]string{
		"/masterchainInfo": `{"last":{"seqno":84467002,"workchain":0}}`,
	})
	if _, err := wrong.BlockNumber(context.Background()); err == nil {
		t.Fatal("accepted a basechain seqno as the masterchain head — shard seqnos are not a global sequence")
	}
}
