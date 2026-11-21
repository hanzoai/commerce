// Package x402 implements the x402 Payment Protocol (LP-3028) for HTTP-native
// crypto payments using the 402 Payment Required status code.
//
// The protocol enables AI agents and applications to make instant on-chain
// payments without human intervention, using USDC (or any ERC-3009 compatible
// token) on EVM chains.
//
// Flow:
//  1. Client requests a resource.
//  2. Server responds with 402 and an X-Payment-Request header describing the payment.
//  3. Client signs an ERC-3009 transferWithAuthorization and retries with X-Payment-Authorization.
//  4. Server verifies the authorization via the facilitator and serves the resource.
package x402

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"
)

const (
	// Version is the x402 protocol version.
	Version = "1"

	// HeaderPaymentRequest is the header sent in 402 responses with payment details.
	HeaderPaymentRequest = "X-Payment-Request"

	// HeaderPaymentAuthorization is the header sent by clients with signed payment auth.
	HeaderPaymentAuthorization = "X-Payment-Authorization"

	// HeaderPaymentReceipt is the header sent back to clients after successful payment.
	HeaderPaymentReceipt = "X-Payment-Receipt"

	// DefaultValidFor is the default payment validity window in seconds (5 minutes).
	DefaultValidFor = 300

	// DefaultNetwork is the default settlement network.
	DefaultNetwork = "lux"

	// DefaultChainID is the Lux mainnet chain ID.
	DefaultChainID = 96369

	// USDCDecimals is the number of decimals for USDC.
	USDCDecimals = 6
)

// PaymentRequest is the server's payment demand, sent in the 402 response body
// and X-Payment-Request header. It tells the client exactly what to pay.
type PaymentRequest struct {
	Version     string `json:"version"`
	Network     string `json:"network"`
	ChainID     int64  `json:"chainId"`
	Facilitator string `json:"facilitator"` // Facilitator contract address
	Payee       string `json:"payee"`       // Merchant/resource owner address
	Token       string `json:"token"`       // Token contract address (e.g., USDC)
	Amount      string `json:"amount"`      // Amount in token's smallest unit
	Resource    string `json:"resource"`    // The resource path being paid for
	ValidFor    int64  `json:"validFor"`    // Validity window in seconds
}

// PaymentAuthorization is the client's signed payment, sent via
// X-Payment-Authorization header. Contains an ERC-3009 transferWithAuthorization.
type PaymentAuthorization struct {
	From        string `json:"from"`        // Payer address
	To          string `json:"to"`          // Facilitator address
	Value       string `json:"value"`       // Amount
	ValidAfter  int64  `json:"validAfter"`  // Unix timestamp
	ValidBefore int64  `json:"validBefore"` // Unix timestamp
	Nonce       string `json:"nonce"`       // Random 32-byte nonce (hex)
	Signature   string `json:"signature"`   // EIP-712 signature (hex)
}

// PaymentReceipt is returned by the facilitator after successful payment settlement.
type PaymentReceipt struct {
	RequestHash string `json:"requestHash"`
	Payer       string `json:"payer"`
	Payee       string `json:"payee"`
	Amount      string `json:"amount"`
	TxHash      string `json:"txHash"`
	Timestamp   int64  `json:"timestamp"`
	Success     bool   `json:"success"`
}

// RouteConfig defines x402 payment requirements for a specific route.
type RouteConfig struct {
	Path        string // URL path pattern
	Payee       string // Merchant address for this route
	Token       string // Token address (default: USDC)
	Amount      string // Amount in smallest token unit
	Network     string // Network name (default: "lux")
	ChainID     int64  // Chain ID (default: 96369)
	Facilitator string // Facilitator contract address
	ValidFor    int64  // Validity window in seconds
}

// PaywallConfig holds the configuration for the x402 paywall middleware.
type PaywallConfig struct {
	// Facilitator is the default facilitator contract address.
	Facilitator string

	// Payee is the default payee address (merchant).
	Payee string

	// Token is the default payment token address (USDC).
	Token string

	// Network is the default network name.
	Network string

	// ChainID is the default chain ID.
	ChainID int64

	// Routes maps path patterns to their payment configuration.
	// If a path is not in this map, it passes through without payment.
	Routes map[string]*RouteConfig

	// FacilitatorURL is the URL of the facilitator service for settlement.
	FacilitatorURL string
}

// NewPaymentRequest creates a PaymentRequest for the given route and resource path.
func NewPaymentRequest(cfg *PaywallConfig, route *RouteConfig, resourcePath string) *PaymentRequest {
	network := route.Network
	if network == "" {
		network = cfg.Network
	}
	if network == "" {
		network = DefaultNetwork
	}

	chainID := route.ChainID
	if chainID == 0 {
		chainID = cfg.ChainID
	}
	if chainID == 0 {
		chainID = DefaultChainID
	}

	facilitator := route.Facilitator
	if facilitator == "" {
		facilitator = cfg.Facilitator
	}

	payee := route.Payee
	if payee == "" {
		payee = cfg.Payee
	}

	token := route.Token
	if token == "" {
		token = cfg.Token
	}

	validFor := route.ValidFor
	if validFor == 0 {
		validFor = DefaultValidFor
	}

	return &PaymentRequest{
		Version:     Version,
		Network:     network,
		ChainID:     chainID,
		Facilitator: facilitator,
		Payee:       payee,
		Token:       token,
		Amount:      route.Amount,
		Resource:    resourcePath,
		ValidFor:    validFor,
	}
}

// MarshalHeader serializes a PaymentRequest for use in an HTTP header.
func (pr *PaymentRequest) MarshalHeader() string {
	data, _ := json.Marshal(pr)
	return string(data)
}

// ParsePaymentRequest parses a PaymentRequest from an HTTP header value.
func ParsePaymentRequest(header string) (*PaymentRequest, error) {
	var pr PaymentRequest
	if err := json.Unmarshal([]byte(header), &pr); err != nil {
		return nil, fmt.Errorf("x402: invalid payment request header: %w", err)
	}
	if pr.Version == "" || pr.Amount == "" || pr.Facilitator == "" {
		return nil, fmt.Errorf("x402: incomplete payment request: version, amount, and facilitator are required")
	}
	return &pr, nil
}

// MarshalHeader serializes a PaymentAuthorization for use in an HTTP header.
func (pa *PaymentAuthorization) MarshalHeader() string {
	data, _ := json.Marshal(pa)
	return string(data)
}

// ParsePaymentAuthorization parses a PaymentAuthorization from an HTTP header value.
func ParsePaymentAuthorization(header string) (*PaymentAuthorization, error) {
	var pa PaymentAuthorization
	if err := json.Unmarshal([]byte(header), &pa); err != nil {
		return nil, fmt.Errorf("x402: invalid payment authorization header: %w", err)
	}
	if pa.From == "" || pa.Signature == "" {
		return nil, fmt.Errorf("x402: incomplete payment authorization: from and signature are required")
	}
	return &pa, nil
}

// IsExpired returns true if the authorization has expired.
func (pa *PaymentAuthorization) IsExpired() bool {
	return time.Now().Unix() > pa.ValidBefore
}

// IsNotYetValid returns true if the authorization is not yet valid.
func (pa *PaymentAuthorization) IsNotYetValid() bool {
	return time.Now().Unix() < pa.ValidAfter
}

// USDCAmount converts a USD cents amount to USDC smallest unit (6 decimals).
// For example, 100 cents ($1.00) becomes "1000000".
func USDCAmount(cents int64) string {
	// 1 USDC = 1,000,000 units (6 decimals)
	// 1 cent = 10,000 units
	units := new(big.Int).Mul(big.NewInt(cents), big.NewInt(10000))
	return units.String()
}

// digest is the EIP-712 hash of this authorization under the token's domain
// (name, version, the request's chain and token): the one EIP-3009 encoding,
// with every field read strictly.
func (pa *PaymentAuthorization) digest(name, version string, req *PaymentRequest) ([32]byte, error) {
	if pa.ValidAfter < 0 || pa.ValidBefore < 0 {
		return [32]byte{}, fmt.Errorf("x402: negative validity window")
	}
	p, err := Authorization{
		From:        pa.From,
		To:          pa.To,
		Value:       pa.Value,
		ValidAfter:  strconv.FormatInt(pa.ValidAfter, 10),
		ValidBefore: strconv.FormatInt(pa.ValidBefore, 10),
		Nonce:       pa.Nonce,
	}.Parse()
	if err != nil {
		return [32]byte{}, fmt.Errorf("x402: authorization: %w", err)
	}
	return Domain{Name: name, Version: version, ChainID: req.ChainID, VerifyingContract: req.Token}.Digest(p)
}
