package catalogentry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

// OpenRouter is the third-party upstream we resell. This file is the ONE place
// its wire format is known: everything past DecodeOpenRouter is a ModelRow, so a
// second upstream (a family service, a direct vendor) is a second decoder and no
// change at all to the sync, the store or the projection.

// ServesOpenRouter is the family that answers an OpenRouter-sourced slug, and
// the SCOPE of an OpenRouter sync run — see Refresh, which only ever withdraws
// models this upstream is responsible for.
const ServesOpenRouter = "openrouter"

// OpenRouterURL is the upstream catalog endpoint.
const OpenRouterURL = "https://openrouter.ai/api/v1/models"

// openRouterModel is OpenRouter's model record — only the fields we consume.
// Prices are exact decimal strings in USD PER TOKEN, which is why they are read
// as strings and shifted six places rather than parsed as numbers.
type openRouterModel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	ContextLength int    `json:"context_length"`
	Architecture  struct {
		Modality        string   `json:"modality"`
		InputModalities []string `json:"input_modalities"`
	} `json:"architecture"`
	Pricing struct {
		Prompt          string `json:"prompt"`
		Completion      string `json:"completion"`
		InputCacheRead  string `json:"input_cache_read"`
		InputCacheWrite string `json:"input_cache_write"`
	} `json:"pricing"`
	TopProvider struct {
		MaxCompletionTokens int `json:"max_completion_tokens"`
	} `json:"top_provider"`
	SupportedParameters []string `json:"supported_parameters"`
}

// DecodeOpenRouter parses an OpenRouter /models payload into ModelRows.
//
// It fails on a malformed or empty body rather than returning what it managed to
// read. A half-decoded catalog looks exactly like an upstream that dropped two
// hundred models, and the sync's answer to models disappearing is to withdraw
// them — so "upstream shrank" and "we failed to read it" have to be told apart
// HERE, because after this function they are indistinguishable.
func DecodeOpenRouter(body []byte) ([]ModelRow, error) {
	var payload struct {
		Data []openRouterModel `json:"data"`
	}
	if err := json.DecodeBytes(body, &payload); err != nil {
		return nil, fmt.Errorf("decode openrouter catalog: %w", err)
	}
	if len(payload.Data) == 0 {
		return nil, errors.New("openrouter catalog decoded to zero models")
	}

	// Vendor is a property of the NAMESPACE, not of one model's display string,
	// so it is resolved once for the whole payload. Deriving it per model gives
	// "OpenAI" and "Openai" for the same party, which makes grouping by vendor
	// quietly wrong.
	vendors := resolveVendors(payload.Data)

	rows := make([]ModelRow, 0, len(payload.Data))
	for i, m := range payload.Data {
		if m.ID == "" {
			continue
		}
		_, name := SplitVendor(m.Name)
		meta := Map{}
		if strings.HasPrefix(m.ID, "~") {
			// OpenRouter's "~vendor/x-latest" aliases float: the id does not pin
			// a version, so what it resolves to — and what it costs — can change
			// while the id stays the same. Listed like everything else, and
			// marked, because a cost that moves under a stable id is precisely
			// the surprise this catalog exists to make legible.
			meta["floating"] = true
		}
		rows = append(rows, ModelRow{
			Slug:        m.ID,
			Name:        name,
			Description: firstSentence(m.Description),
			Order:       i,
			Metadata:    meta,
			Costs:       openRouterCosts(m),
			Spec: ModelSpec{
				Vendor:        vendors[Namespace(m.ID)],
				Family:        FamilyThirdParty,
				Serves:        ServesOpenRouter,
				Upstream:      m.ID,
				Modality:      modality(m),
				ContextWindow: m.ContextLength,
				MaxOutput:     m.TopProvider.MaxCompletionTokens,
				Capabilities:  capabilities(m),
				Enabled:       true,
			},
		})
	}
	if len(rows) == 0 {
		return nil, errors.New("openrouter catalog yielded no usable models")
	}
	return rows, nil
}

// openRouterCosts converts the per-TOKEN upstream figures into per-MILLION-token
// costs by shifting the decimal point six places — exact, no float anywhere.
//
// A component the upstream prices at zero is OMITTED rather than recorded as a
// cost of "0": free is a real upstream tier, and a stored zero cost would read
// as a 100% margin on anything we charge for it. A model with no priced
// component at all keeps its row — it is still a model, and this catalog lists
// every model — so it simply carries no rates, which reads as "nothing metered".
func openRouterCosts(m openRouterModel) []Rate {
	spec := []struct{ key, amount string }{
		{RateIn, m.Pricing.Prompt},
		{RateOut, m.Pricing.Completion},
		{RateCacheRead, m.Pricing.InputCacheRead},
		{RateCacheWrite, m.Pricing.InputCacheWrite},
	}
	costs := make([]Rate, 0, len(spec))
	for _, s := range spec {
		d, ok := dec(s.amount)
		if !ok || d.IsZero() {
			continue
		}
		costs = append(costs, Rate{
			Key:  s.key,
			Unit: UnitMTok,
			Cost: d.Shift(6).String(), // per token → per million tokens, exactly
		})
	}
	return costs
}

// modality maps OpenRouter's "text+image->text" into the ModelSpec vocabulary.
func modality(m openRouterModel) string {
	_, out, ok := strings.Cut(m.Architecture.Modality, "->")
	if !ok {
		return "chat"
	}
	switch strings.TrimSpace(out) {
	case "image":
		return "image"
	case "audio":
		return "audio"
	case "video":
		return "video"
	case "embedding":
		return "embedding"
	default:
		return "chat"
	}
}

// capabilities reports the machine-observable ones, in a stable order so an
// unchanged upstream produces an unchanged row.
func capabilities(m openRouterModel) []string {
	var caps []string
	for _, mod := range m.Architecture.InputModalities {
		if mod == "image" {
			caps = append(caps, "vision")
			break
		}
	}
	for _, p := range m.SupportedParameters {
		if p == "tools" {
			caps = append(caps, "tools")
			break
		}
	}
	return caps
}

// firstSentence trims an upstream description to a one-liner for the catalog.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, ". "); i > 0 && i < 240 {
		return s[:i+1]
	}
	if len(s) > 240 {
		if cut := strings.LastIndexByte(s[:240], ' '); cut > 0 {
			return s[:cut] + "…"
		}
		return s[:240] + "…"
	}
	return s
}

// Namespace is the vendor segment of a model slug, with OpenRouter's floating
// "~" alias marker removed so "~openai" and "openai" are ONE vendor.
func Namespace(slug string) string {
	ns, _, ok := strings.Cut(slug, "/")
	if !ok {
		return ""
	}
	return strings.TrimPrefix(ns, "~")
}

// SplitVendor decomplects an upstream display name into the PARTY and the
// PRODUCT: "Meta: Llama 4 Maverick" becomes "Meta" and "Llama 4 Maverick".
//
// Carrying "Meta Llama" as one string is the vendor+product conflation we do not
// want: it makes grouping by vendor impossible without re-parsing a display
// string, and it reads as though the vendor were named after its product. The
// vendor here is only THIS model's opinion; resolveVendors settles the
// namespace's actual name across the whole payload.
func SplitVendor(display string) (vendor, name string) {
	display = strings.TrimSpace(display)
	if head, rest, ok := strings.Cut(display, ": "); ok {
		head, rest = strings.TrimSpace(head), strings.TrimSpace(rest)
		// A vendor is a short proper noun. Anything longer is part of a model
		// name that happened to contain a colon.
		if head != "" && rest != "" && len(head) <= 24 {
			return head, rest
		}
	}
	return "", display
}

// resolveVendors names every vendor in a payload, once, from the upstream's own
// display prefixes: the most frequent prefix a namespace publishes wins, ties
// broken alphabetically so a run is deterministic.
//
// This is a MECHANISM rather than a table, which is the point — a vendor
// OpenRouter adds tomorrow is named correctly with no edit here, and a vendor
// that renames itself follows. vendorNames covers only the handful of namespaces
// that publish no prefix on any model; a long hardcoded list of vendor names
// would be one more thing to rot.
func resolveVendors(models []openRouterModel) map[string]string {
	votes := map[string]map[string]int{}
	for _, m := range models {
		ns := Namespace(m.ID)
		if ns == "" {
			continue
		}
		if _, ok := votes[ns]; !ok {
			votes[ns] = map[string]int{}
		}
		if v, _ := SplitVendor(m.Name); v != "" {
			votes[ns][v]++
		}
	}
	out := make(map[string]string, len(votes))
	for ns, tally := range votes {
		best, bestN := "", 0
		for name, n := range tally {
			if n > bestN || (n == bestN && name < best) {
				best, bestN = name, n
			}
		}
		if best == "" {
			best = vendorFromNamespace(ns)
		}
		out[ns] = best
	}
	return out
}

// vendorFromNamespace names the party from the slug's namespace alone, for a
// vendor that publishes no display prefix on any of its models.
func vendorFromNamespace(ns string) string {
	if ns == "" {
		return ""
	}
	if name, ok := vendorNames[ns]; ok {
		return name
	}
	parts := strings.FieldsFunc(ns, func(r rune) bool { return r == '-' || r == '_' })
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// vendorNames corrects the namespaces that publish no display prefix at all, so
// the title-case fallback does not produce something like "Anthracite Org".
var vendorNames = map[string]string{
	"anthracite-org": "Anthracite",
	"gryphe":         "Gryphe",
	"openrouter":     "OpenRouter",
	"rekaai":         "Reka",
	"undi95":         "Undi95",
}

// FetchOpenRouter reads the live upstream catalog. It is the ONLY I/O in the
// sync path, deliberately split from DecodeOpenRouter so every rule about what a
// payload MEANS is testable against a fixture with no network.
//
// The API key is optional (the catalog is public) but is sent when present so
// the request is attributed to our account.
func FetchOpenRouter(ctx context.Context, apiKey string) ([]ModelRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, OpenRouterURL, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch openrouter catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter catalog: upstream returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("read openrouter catalog: %w", err)
	}
	return DecodeOpenRouter(body)
}
