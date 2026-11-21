// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Profile is a UCP profile document: the business profile served at
// /.well-known/ucp or the platform profile a request's UCP-Agent header names
// (overview, "Profile Structure"; schemas/profile.json). Its keys[] is an RFC
// 7517 JWK Set, so the same document is also a valid JWK Set.
type Profile struct {
	UCP  Meta  `json:"ucp"`
	Keys []Key `json:"keys,omitempty"`
}

// Meta is the ucp member: protocol metadata (schemas/ucp.json). A nil registry
// is absent; an empty one is present, which a profile's services and
// payment_handlers must be even when empty ("Profile Structure"). The member is
// open: members it does not define are ignored.
type Meta struct {
	Version string
	// SupportedVersions maps each older protocol version a business supports to
	// the URI of its own complete profile ("Protocol Version").
	SupportedVersions map[string]string
	// Status is "success" or "error" in responses.
	Status string
	// MapOrder carries the preferred key order of the registries ("The ucp
	// Protocol Namespace", map_order).
	MapOrder        map[string][]string
	Services        map[string][]Service
	Capabilities    map[string][]Capability
	PaymentHandlers map[string][]Handler
	// RequestConstraints is the response-only request_constraints member.
	RequestConstraints json.RawMessage
}

type metaWire struct {
	Version            string                   `json:"version"`
	SupportedVersions  map[string]string        `json:"supported_versions,omitempty"`
	Status             string                   `json:"status,omitempty"`
	MapOrder           map[string][]string      `json:"map_order,omitempty"`
	Services           *map[string][]Service    `json:"services,omitempty"`
	Capabilities       *map[string][]Capability `json:"capabilities,omitempty"`
	PaymentHandlers    *map[string][]Handler    `json:"payment_handlers,omitempty"`
	RequestConstraints json.RawMessage          `json:"request_constraints,omitempty"`
}

func (m Meta) MarshalJSON() ([]byte, error) {
	w := metaWire{Version: m.Version, SupportedVersions: m.SupportedVersions, Status: m.Status,
		MapOrder: m.MapOrder, RequestConstraints: m.RequestConstraints}
	if m.Services != nil {
		w.Services = &m.Services
	}
	if m.Capabilities != nil {
		w.Capabilities = &m.Capabilities
	}
	if m.PaymentHandlers != nil {
		w.PaymentHandlers = &m.PaymentHandlers
	}
	return json.Marshal(w)
}

func (m *Meta) UnmarshalJSON(b []byte) error {
	var w metaWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*m = Meta{Version: w.Version, SupportedVersions: w.SupportedVersions, Status: w.Status,
		MapOrder: w.MapOrder, RequestConstraints: w.RequestConstraints}
	if w.Services != nil {
		m.Services = *w.Services
		if m.Services == nil {
			m.Services = map[string][]Service{}
		}
	}
	if w.Capabilities != nil {
		m.Capabilities = *w.Capabilities
		if m.Capabilities == nil {
			m.Capabilities = map[string][]Capability{}
		}
	}
	if w.PaymentHandlers != nil {
		m.PaymentHandlers = *w.PaymentHandlers
		if m.PaymentHandlers == nil {
			m.PaymentHandlers = map[string][]Handler{}
		}
	}
	return nil
}

// Entity is the shared foundation of services, capabilities and payment
// handlers (schemas/ucp.json, entity).
type Entity struct {
	Version string          `json:"version"`
	Spec    string          `json:"spec,omitempty"`
	Schema  string          `json:"schema,omitempty"`
	ID      string          `json:"id,omitempty"`
	Config  json.RawMessage `json:"config,omitempty"`
}

// Service is one transport binding of a service (schemas/service.json).
type Service struct {
	Entity
	Transport string `json:"transport"`
	Endpoint  string `json:"endpoint,omitempty"`
}

// Capability is a capability or, with Extends, an extension (schemas/capability.json).
type Capability struct {
	Entity
	// Extends is a parent capability name or an array of them.
	Extends json.RawMessage `json:"extends,omitempty"`
}

// Parents returns the capabilities this one extends; none for a root capability.
func (c Capability) Parents() []string {
	if len(c.Extends) == 0 {
		return nil
	}
	var one string
	if json.Unmarshal(c.Extends, &one) == nil {
		return []string{one}
	}
	var many []string
	if json.Unmarshal(c.Extends, &many) == nil {
		return many
	}
	return nil
}

// Handler is one payment handler instance (schemas/payment_handler.json).
type Handler struct {
	Entity
	AvailableInstruments []AvailableInstrument `json:"available_instruments,omitempty"`
}

// AvailableInstrument is an instrument type a handler supports, with optional
// constraints (schemas/common/types/available_payment_instrument.json).
type AvailableInstrument struct {
	Type        string          `json:"type"`
	Constraints json.RawMessage `json:"constraints,omitempty"`
	UCP         json.RawMessage `json:"ucp,omitempty"`
}

// Extends names one parent as the extends member.
func Extends(parents ...string) json.RawMessage {
	var b []byte
	if len(parents) == 1 {
		b, _ = json.Marshal(parents[0])
	} else {
		b, _ = json.Marshal(parents)
	}
	return b
}

var (
	versionRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	// nameRe is schemas/common/types/reverse_domain_name.json.
	nameRe = regexp.MustCompile(`^[a-z](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9_-]*[a-z0-9_])?)+$`)

	errNotHTTPS = errors.New("URL must use https")
	errUserinfo = errors.New("URL must not contain userinfo")
	errHost     = errors.New("URL host must be a registered domain name of at least two labels")
)

func isIPLiteral(h string) bool {
	return net.ParseIP(strings.Trim(h, "[]")) != nil
}

// validVersion reports whether v is a YYYY-MM-DD date.
func validVersion(v string) bool {
	if !versionRe.MatchString(v) {
		return false
	}
	_, err := time.Parse("2006-01-02", v)
	return err == nil
}

// Bind checks the authority binding of an entity's schema URL to its name
// (overview, "Authority Binding"). The URL must parse, use https, carry no
// userinfo, and have a host of at least two labels that is not an IP literal;
// the reversed host labels must equal the name or be a label-aligned prefix of
// it. The check uses the host as written and does not consult the Public
// Suffix List. A host that is not plain ASCII (an A-label host is) is refused.
func Bind(name, schema string) error {
	u, err := url.Parse(schema)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return errNotHTTPS
	}
	if u.User != nil {
		return errUserinfo
	}
	h := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if h == "" || !strings.Contains(h, ".") || isIPLiteral(h) || strings.ContainsAny(h, "\x00 ") {
		return errHost
	}
	for i := 0; i < len(h); i++ {
		if h[i] >= 0x80 {
			return errHost
		}
	}
	labels := strings.Split(h, ".")
	for i, j := 0, len(labels)-1; i < j; i, j = i+1, j-1 {
		labels[i], labels[j] = labels[j], labels[i]
	}
	prefix := strings.Join(labels, ".")
	if name == prefix || strings.HasPrefix(name, prefix+".") {
		return nil
	}
	return fmt.Errorf("schema host %q does not bind name %q", h, name)
}

// Problem is one reason part of a profile is not acceptable.
type Problem struct {
	Path   string
	Reason string
}

func (p Problem) String() string { return p.Path + ": " + p.Reason }

// ProfileError lists everything wrong with a profile document.
type ProfileError struct{ Problems []Problem }

func (e *ProfileError) Error() string {
	s := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		s[i] = p.String()
	}
	return "invalid profile: " + strings.Join(s, "; ")
}

// Kind is the profile variant being validated.
type Kind int

const (
	// Business is the profile a business serves at /.well-known/ucp
	// (schemas/ucp.json, business_schema).
	Business Kind = iota
	// Platform is the profile a platform advertises in UCP-Agent (platform_schema).
	Platform
)

func (k Kind) String() string {
	if k == Platform {
		return "platform"
	}
	return "business"
}

// Validate checks a profile document against the schema variant for kind and
// the normative rules of the overview: required members, registry key names,
// entity fields, the date format of versions, authority binding of every
// declared schema, the version `D` of every dev.ucp.* service and capability,
// well-formed extends, keys[] with no private material, supported_versions.
// It returns a *ProfileError.
func (p *Profile) Validate(kind Kind) error {
	pr := p.problems(kind, true)
	if len(pr) == 0 {
		return nil
	}
	return &ProfileError{Problems: pr}
}

// Wellformed is Validate without the checks that reject single entities:
// authority binding and the dev.ucp.* version rule. A document that is not
// well formed is profile_malformed; one that is, with entities Admit rejects,
// is still usable without them.
func (p *Profile) Wellformed(kind Kind) error {
	pr := p.problems(kind, false)
	if len(pr) == 0 {
		return nil
	}
	return &ProfileError{Problems: pr}
}

// isUCP reports whether name is in the dev.ucp.* namespace.
func isUCP(name string) bool { return strings.HasPrefix(name, "dev.ucp.") }

func (p *Profile) problems(kind Kind, entities bool) []Problem {
	var out []Problem
	add := func(path, format string, args ...any) { out = append(out, Problem{path, fmt.Sprintf(format, args...)}) }
	m := p.UCP
	if !validVersion(m.Version) {
		add("ucp.version", "must be a YYYY-MM-DD date, got %q", m.Version)
	}
	if m.Services == nil {
		add("ucp.services", "is required, even when empty")
	}
	if m.PaymentHandlers == nil {
		add("ucp.payment_handlers", "is required, even when empty")
	}
	if len(m.SupportedVersions) > 0 && kind == Platform {
		add("ucp.supported_versions", "belongs to a business profile")
	}
	for v, u := range m.SupportedVersions {
		if !validVersion(v) {
			add("ucp.supported_versions", "key %q is not a YYYY-MM-DD date", v)
		}
		if v == m.Version {
			add("ucp.supported_versions", "lists the profile's own version %s", v)
		}
		if err := checkURL(u); err != nil {
			add("ucp.supported_versions", "%s: %v", v, err)
		}
	}
	entity := func(path, name string, e Entity, spec, schema bool) {
		if !validVersion(e.Version) {
			add(path+".version", "must be a YYYY-MM-DD date, got %q", e.Version)
		}
		if e.Spec != "" {
			if u, err := url.Parse(e.Spec); err != nil || u.Scheme != "https" {
				add(path+".spec", "must be an https URL")
			}
		} else if spec {
			add(path+".spec", "is required")
		}
		if e.Schema == "" && schema {
			add(path+".schema", "is required")
		}
		if e.Schema != "" && entities {
			if err := Bind(name, e.Schema); err != nil {
				add(path+".schema", "%v", err)
			}
		}
	}
	for _, name := range sortedKeys(m.Services) {
		if !nameRe.MatchString(name) {
			add("ucp.services", "key %q is not a reverse-domain name", name)
		}
		for i, s := range m.Services[name] {
			path := fmt.Sprintf("ucp.services[%q][%d]", name, i)
			needSchema := kind == Platform && s.Transport != "a2a"
			entity(path, name, s.Entity, kind == Platform, needSchema)
			if entities && isUCP(name) && s.Version != m.Version {
				add(path+".version", "a dev.ucp.* entry must declare ucp.version %s", m.Version)
			}
			switch s.Transport {
			case "rest", "mcp", "a2a", "embedded":
			default:
				add(path+".transport", "must be rest, mcp, a2a or embedded, got %q", s.Transport)
			}
			if s.Endpoint != "" {
				if err := checkEndpoint(s.Endpoint); err != nil {
					add(path+".endpoint", "%v", err)
				}
			} else if kind == Business && s.Transport != "embedded" && s.Transport != "" {
				add(path+".endpoint", "is required for transport %s", s.Transport)
			}
		}
	}
	for _, name := range sortedKeys(m.Capabilities) {
		if !nameRe.MatchString(name) {
			add("ucp.capabilities", "key %q is not a reverse-domain name", name)
		}
		for i, c := range m.Capabilities[name] {
			path := fmt.Sprintf("ucp.capabilities[%q][%d]", name, i)
			entity(path, name, c.Entity, kind == Platform, true)
			if entities && isUCP(name) && c.Version != m.Version {
				add(path+".version", "a dev.ucp.* entry must declare ucp.version %s", m.Version)
			}
			if len(c.Extends) > 0 {
				ps := c.Parents()
				if len(ps) == 0 {
					add(path+".extends", "must be a capability name or a non-empty array of them")
				}
				for _, par := range ps {
					if !nameRe.MatchString(par) {
						add(path+".extends", "%q is not a reverse-domain name", par)
					}
				}
			}
		}
	}
	for _, name := range sortedKeys(m.PaymentHandlers) {
		if !nameRe.MatchString(name) {
			add("ucp.payment_handlers", "key %q is not a reverse-domain name", name)
		}
		for i, h := range m.PaymentHandlers[name] {
			path := fmt.Sprintf("ucp.payment_handlers[%q][%d]", name, i)
			entity(path, name, h.Entity, kind == Platform, kind == Platform)
			if h.ID == "" {
				add(path+".id", "is required")
			}
			if h.AvailableInstruments != nil && len(h.AvailableInstruments) == 0 {
				add(path+".available_instruments", "must not be empty when present")
			}
			for j, ai := range h.AvailableInstruments {
				if ai.Type == "" {
					add(fmt.Sprintf("%s.available_instruments[%d].type", path, j), "is required")
				}
			}
		}
	}
	seen := map[string]bool{}
	for i, k := range p.Keys {
		if err := k.check(); err != nil {
			add(fmt.Sprintf("keys[%d]", i), "%v", err)
		}
		if seen[k.KID] {
			add(fmt.Sprintf("keys[%d]", i), "kid %q appears twice", k.KID)
		}
		seen[k.KID] = true
	}
	return out
}

// checkEndpoint requires a valid https URL for an endpoint ("Endpoint
// Resolution"); a trailing slash is discouraged and is refused so that the
// OpenAPI paths append directly.
func checkEndpoint(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || u.Host == "" {
		return errors.New("must be an https URL")
	}
	if strings.HasSuffix(u.Path, "/") {
		return errors.New("must not end with a slash")
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Admit removes the entities of a received profile that must be treated as not
// present: a capability, service or payment handler whose schema URL does not
// bind to its name, and a dev.ucp.* service or capability whose version is not
// the profile's ucp.version (overview, "Enforcement" and "Component Versioning
// and Release Snapshots"). It returns what it removed. A profile without
// entities left in a registry keeps the registry empty.
func (p *Profile) Admit() []Problem {
	var out []Problem
	drop := func(path, why string) { out = append(out, Problem{path, why}) }
	m := &p.UCP
	for name, list := range m.Services {
		var keep []Service
		for i, s := range list {
			path := fmt.Sprintf("ucp.services[%q][%d]", name, i)
			switch {
			case s.Schema != "" && Bind(name, s.Schema) != nil:
				drop(path, "schema does not bind to the name")
			case isUCP(name) && s.Version != m.Version:
				drop(path, "dev.ucp.* version differs from ucp.version")
			default:
				keep = append(keep, s)
			}
		}
		m.Services[name] = keep
	}
	for name, list := range m.Capabilities {
		var keep []Capability
		for i, c := range list {
			path := fmt.Sprintf("ucp.capabilities[%q][%d]", name, i)
			switch {
			case c.Schema != "" && Bind(name, c.Schema) != nil:
				drop(path, "schema does not bind to the name")
			case isUCP(name) && c.Version != m.Version:
				drop(path, "dev.ucp.* version differs from ucp.version")
			default:
				keep = append(keep, c)
			}
		}
		if len(keep) == 0 {
			delete(m.Capabilities, name)
			continue
		}
		m.Capabilities[name] = keep
	}
	for name, list := range m.PaymentHandlers {
		var keep []Handler
		for i, h := range list {
			if h.Schema != "" && Bind(name, h.Schema) != nil {
				drop(fmt.Sprintf("ucp.payment_handlers[%q][%d]", name, i), "schema does not bind to the name")
				continue
			}
			keep = append(keep, h)
		}
		if len(keep) == 0 {
			delete(m.PaymentHandlers, name)
			continue
		}
		m.PaymentHandlers[name] = keep
	}
	return out
}

// Ordered returns the keys of a registry in the order map_order prefers: the
// listed keys that are present first, in array order with later repeats
// ignored, then the remaining keys in RFC 8785 property-name order (for ASCII
// names, byte order). An entry that names an absent key is ignored (overview,
// "map_order").
func Ordered[V any](reg map[string]V, order []string) []string {
	out := make([]string, 0, len(reg))
	seen := map[string]bool{}
	for _, k := range order {
		if _, ok := reg[k]; ok && !seen[k] {
			out = append(out, k)
			seen[k] = true
		}
	}
	for _, k := range sortedKeys(reg) {
		if !seen[k] {
			out = append(out, k)
		}
	}
	return out
}

// Standard URLs of the dev.ucp.* entities for a release (overview,
// "Business Profile").
func ucpURL(kind, path string) string {
	return "https://ucp.dev/" + Version + "/" + kind + "/" + path
}

func ucpEntity() Entity { return Entity{Version: Version} }

// ProfileConfig is what a host states about its business.
type ProfileConfig struct {
	// Endpoint is the base URL of the REST binding: https, no trailing slash.
	// The REST paths of the specification are appended to it.
	Endpoint string
	// Handlers are the payment handlers the business advertises, keyed by
	// reverse-domain handler name. They are the host's: the specification
	// defines the declaration structure and no concrete handler.
	Handlers map[string][]Handler
	// Keys are the public keys the business signs responses and webhooks with.
	Keys []Key
	// Order advertises the Order capability.
	Order bool
	// MapOrder is the preferred order of payment handlers on presentation.
	HandlerOrder []string
}

// NewProfile builds the business profile for cfg and validates it.
func NewProfile(cfg ProfileConfig) (*Profile, error) {
	svc := Service{Entity: Entity{Version: Version, Spec: ucpURL("specification", "overview/"),
		Schema: ucpURL("services", "shopping/rest.openapi.json")}, Transport: TransportR, Endpoint: cfg.Endpoint}
	caps := map[string][]Capability{
		Checkout: {{Entity: Entity{Version: Version, Spec: ucpURL("specification", "shopping/checkout"),
			Schema: ucpURL("schemas", "shopping/checkout.json")}}},
	}
	if cfg.Order {
		caps[OrderCap] = []Capability{{Entity: Entity{Version: Version, Spec: ucpURL("specification", "shopping/order"),
			Schema: ucpURL("schemas", "shopping/order.json")}}}
	}
	handlers := cfg.Handlers
	if handlers == nil {
		handlers = map[string][]Handler{}
	}
	p := &Profile{
		UCP:  Meta{Version: Version, Services: map[string][]Service{Shopping: {svc}}, Capabilities: caps, PaymentHandlers: handlers},
		Keys: cfg.Keys,
	}
	if len(cfg.HandlerOrder) > 0 {
		p.UCP.MapOrder = map[string][]string{"payment_handlers": cfg.HandlerOrder}
	}
	if err := p.Validate(Business); err != nil {
		return nil, err
	}
	return p, nil
}
