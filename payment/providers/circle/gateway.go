// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package circle

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// The unified-balance half of Gateway (gateway.yaml): what a depositor holds,
// what is still arriving, and what a cross-chain transfer out of it costs and
// became. The commerce rail reads the first two for the treasury view; the
// last two are the outbound path's reads (LLM.md, "Circle Gateway").

// USDC is the only token the commerce rail credits: it is pegged 1:1 to the
// dollar the ledger counts in, so no price is involved. EURC and cirBTC are
// Gateway tokens too and would need one.
const USDC = "USDC"

// Info is GET /v1/info: the domains Gateway serves and their contracts.
type Info struct {
	Version int     `json:"version"`
	Domains []Chain `json:"domains"`
}

// Chain is one Gateway domain. Domain is Circle's identifier (0 Ethereum,
// 6 Base, …), not an EVM chain id.
type Chain struct {
	Chain            string    `json:"chain"`
	Network          string    `json:"network"`
	Domain           uint32    `json:"domain"`
	Wallet           *Contract `json:"walletContract,omitempty"`
	Minter           *Contract `json:"minterContract,omitempty"`
	ProcessedHeight  string    `json:"processedHeight"`
	ExpirationHeight string    `json:"burnIntentExpirationHeight"`
}

// Contract is a Gateway contract on one domain and the tokens it handles.
type Contract struct {
	Address string   `json:"address"`
	Tokens  []string `json:"supportedTokens"`
}

// Info reads the domains and contracts Gateway serves.
func (c *Client) Info(ctx context.Context) (Info, error) {
	var out Info
	err := c.do(ctx, http.MethodGet, "/v1/info", nil, nil, &out)
	return out, err
}

// source names a depositor; with no domain Gateway answers for every domain
// the address is valid on.
type source struct {
	Depositor string `json:"depositor"`
}

type sources struct {
	Token   string   `json:"token"`
	Sources []source `json:"sources"`
}

// Balance is a depositor's AVAILABLE balance on one domain, as Gateway writes
// it: decimal token units ("12.500000"), not base units.
type Balance struct {
	Domain    uint32 `json:"domain"`
	Depositor string `json:"depositor"`
	Balance   string `json:"balance"`
}

// Balances reads depositor's available balance of token on every domain.
func (c *Client) Balances(ctx context.Context, token, depositor string) ([]Balance, error) {
	var out struct {
		Token    string    `json:"token"`
		Balances []Balance `json:"balances"`
	}
	err := c.do(ctx, http.MethodPost, "/v1/balances", nil, sources{Token: token, Sources: []source{{Depositor: depositor}}}, &out)
	return out.Balances, err
}

// Deposit is an onchain deposit into a Gateway Wallet that Gateway has seen
// and not yet finalized. Amount is in BASE units.
type Deposit struct {
	Depositor       string `json:"depositor"`
	Domain          uint32 `json:"domain"`
	TransactionHash string `json:"transactionHash"`
	Amount          string `json:"amount"`
	Status          string `json:"status"`
	BlockHeight     string `json:"blockHeight"`
	BlockHash       string `json:"blockHash"`
	BlockTimestamp  string `json:"blockTimestamp"`
}

// Deposits reads depositor's pending deposits of token on every domain.
func (c *Client) Deposits(ctx context.Context, token, depositor string) ([]Deposit, error) {
	var out struct {
		Token    string    `json:"token"`
		Deposits []Deposit `json:"deposits"`
	}
	err := c.do(ctx, http.MethodPost, "/v1/deposits", nil, sources{Token: token, Sources: []source{{Depositor: depositor}}}, &out)
	return out.Deposits, err
}

// Spec is a TransferSpec: one burn on a source domain and the mint it pays for
// on a destination domain. Addresses are bytes32 hex when submitted; Gateway
// answers them in canonical form.
type Spec struct {
	Version              uint32 `json:"version"`
	SourceDomain         uint32 `json:"sourceDomain"`
	DestinationDomain    uint32 `json:"destinationDomain"`
	SourceContract       string `json:"sourceContract"`
	DestinationContract  string `json:"destinationContract"`
	SourceToken          string `json:"sourceToken"`
	DestinationToken     string `json:"destinationToken"`
	SourceDepositor      string `json:"sourceDepositor"`
	DestinationRecipient string `json:"destinationRecipient"`
	SourceSigner         string `json:"sourceSigner"`
	DestinationCaller    string `json:"destinationCaller"`
	Value                string `json:"value"`
	Salt                 string `json:"salt"`
	HookData             string `json:"hookData,omitempty"`
}

// Intent is a burn intent. MaxBlockHeight and MaxFee may be empty on an
// estimate; Gateway fills them.
type Intent struct {
	MaxBlockHeight string `json:"maxBlockHeight,omitempty"`
	MaxFee         string `json:"maxFee,omitempty"`
	Spec           Spec   `json:"spec"`
}

// Fee is one intent's share of a transfer's fees, in decimal token units.
type Fee struct {
	TransferSpecHash string `json:"transferSpecHash"`
	Domain           uint32 `json:"domain"`
	BaseFee          string `json:"baseFee"`
	TransferFee      string `json:"transferFee,omitempty"`
}

// Fees is what a transfer costs, in decimal token units.
type Fees struct {
	Total         string `json:"total"`
	Token         string `json:"token"`
	PerIntent     []Fee  `json:"perIntent"`
	ForwardingFee string `json:"forwardingFee,omitempty"`
}

// Estimate is POST /v1/estimate's answer: the intents with fees and expiry
// heights filled in.
type Estimate struct {
	Body []struct {
		BurnIntent Intent `json:"burnIntent"`
	} `json:"body"`
	Fees Fees `json:"fees"`
}

// Estimate prices burn intents without a signature and without moving money.
func (c *Client) Estimate(ctx context.Context, intents []Intent) (Estimate, error) {
	var out Estimate
	err := c.do(ctx, http.MethodPost, "/v1/estimate", nil, intents, &out)
	return out, err
}

// Transfer is GET /v1/transfer/{id}: a cross-chain transfer created from
// signed burn intents, and how far its destination mint has got.
type Transfer struct {
	DestinationDomain uint32 `json:"destinationDomain"`
	Status            string `json:"status"` // pending | confirmed | finalized | failed | expired
	TransactionHash   string `json:"transactionHash,omitempty"`
	Fees              Fees   `json:"fees"`
}

// Transfer reads one cross-chain transfer by its id.
func (c *Client) Transfer(ctx context.Context, id string) (Transfer, error) {
	var out Transfer
	if !uuid(id) {
		return out, fmt.Errorf("circle gateway: transfer id %q is not a UUID", id)
	}
	err := c.do(ctx, http.MethodGet, "/v1/transfer/"+url.PathEscape(id), nil, nil, &out)
	return out, err
}

// uuid reports whether s is a canonical 8-4-4-4-12 hex UUID. Ids are checked
// before they enter a path so a caller-supplied string can never steer a
// request to another route.
func uuid(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return false
			}
		}
	}
	return true
}
