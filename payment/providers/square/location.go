package square

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/types/currency"
)

// What Square can charge is a fact about the MERCHANT's location, not about a list
// kept here or the buyer's country: a location takes cards in its own currency and
// only while it holds the CREDIT_CARD_PROCESSING capability. The location is read from
// Square (GET /v2/locations/{id}) and re-read every few minutes; a read that fails
// keeps the last answer, and with no answer ever, Square is not offered.

const (
	locationFresh = 10 * time.Minute // how long an answer is believed
	locationRetry = 30 * time.Second // how soon a failed read is asked again

	// LocationStale is how long the last good answer is kept when Square cannot be
	// asked again. Past it Square is ineligible and new checkouts go to Stripe: a
	// location that was disabled while Square was unreachable must not keep taking
	// cards on a belief nobody has checked.
	LocationStale = 30 * time.Minute
)

// Site is the configured Square location as Square states it.
type Site struct {
	ID       string
	Country  string
	Currency currency.Type
	// Cards says the location holds CREDIT_CARD_PROCESSING and is active.
	Cards bool
}

type siteEntry struct {
	site Site
	ok   bool
	at   time.Time     // when this entry was made
	good time.Time     // when Square last answered
	ttl  time.Duration // how long this entry is consulted before asking again
}

var (
	siteMu    sync.Mutex
	siteCache = map[string]*siteEntry{}
)

func (p *Provider) siteKey() string {
	h := sha256.Sum256([]byte(p.config.AccessToken + "\x00" + p.config.LocationID + "\x00" + p.base()))
	return hex.EncodeToString(h[:8])
}

// base is Square's API origin for this provider's environment.
func (p *Provider) base() string {
	if p.config.Endpoint != "" {
		return strings.TrimRight(p.config.Endpoint, "/")
	}
	if p.config.Environment == "sandbox" {
		return "https://connect.squareupsandbox.com"
	}
	return "https://connect.squareup.com"
}

// Location answers the configured location, read from Square when the last answer
// is old. ok is false when Square has never answered for these credentials.
func (p *Provider) Location(ctx context.Context) (Site, bool) {
	if p.config.AccessToken == "" || p.config.LocationID == "" {
		return Site{}, false
	}
	key := p.siteKey()
	siteMu.Lock()
	e := siteCache[key]
	if e != nil && time.Since(e.at) < e.ttl {
		s, ok := e.site, e.ok
		siteMu.Unlock()
		return s, ok
	}
	siteMu.Unlock()

	site, err := p.fetchSite(ctx)
	siteMu.Lock()
	defer siteMu.Unlock()
	now := time.Now()
	switch {
	case err == nil:
		siteCache[key] = &siteEntry{site: site, ok: true, at: now, good: now, ttl: locationFresh}
		return site, true
	case e != nil && e.ok && now.Sub(e.good) < LocationStale:
		log.Warn("square: the location could not be re-read, keeping the last answer (%s old): %v", now.Sub(e.good).Round(time.Second), err)
		e.at, e.ttl = now, locationRetry
		return e.site, true
	default:
		log.Warn("square: the location could not be read and no answer is fresh enough, so Square is not offered: %v", err)
		siteCache[key] = &siteEntry{at: now, ttl: locationRetry}
		return Site{}, false
	}
}

func (p *Provider) fetchSite(ctx context.Context) (Site, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.base()+"/v2/locations/"+p.config.LocationID, nil)
	if err != nil {
		return Site{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.config.AccessToken)
	req.Header.Set("Square-Version", "2024-10-17")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Site{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return Site{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Site{}, &httpError{resp.StatusCode}
	}
	var out struct {
		Location struct {
			ID           string   `json:"id"`
			Country      string   `json:"country"`
			Currency     string   `json:"currency"`
			Status       string   `json:"status"`
			Capabilities []string `json:"capabilities"`
		} `json:"location"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return Site{}, err
	}
	s := Site{ID: out.Location.ID, Country: strings.ToUpper(out.Location.Country), Currency: currency.Type(strings.ToLower(out.Location.Currency))}
	if strings.EqualFold(out.Location.Status, "ACTIVE") {
		for _, c := range out.Location.Capabilities {
			if c == "CREDIT_CARD_PROCESSING" {
				s.Cards = true
			}
		}
	}
	return s, nil
}

type httpError struct{ status int }

func (e *httpError) Error() string { return "square answered " + http.StatusText(e.status) }

// Eligible says Square can take a card payment in cur from this merchant: the
// location holds CREDIT_CARD_PROCESSING and its currency is cur. It answers the
// country the location is in, which the routing policy checks against its Square
// countries.
func (p *Provider) Eligible(ctx context.Context, cur currency.Type) (string, bool) {
	s, ok := p.Location(ctx)
	if !ok || !s.Cards || s.Currency != cur {
		return "", false
	}
	return s.Country, true
}

// Place is one Square location as the admin sees it.
type Place struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Country      string   `json:"country"`
	Currency     string   `json:"currency"`
	Status       string   `json:"status"`
	Capabilities []string `json:"capabilities"`
	// Methods are what a checkout can mount for this location: nothing unless it
	// holds card processing, then a card, Apple Pay, Google Pay, and Cash App Pay
	// where the location is in the United States.
	Methods []string `json:"methods"`
	// Charging marks the location commerce charges against: the one configured.
	Charging bool `json:"charging"`
}

// Locations lists every location the account holds, read from Square now.
func (p *Provider) Locations(ctx context.Context) ([]Place, error) {
	if p.config.AccessToken == "" {
		return []Place{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.base()+"/v2/locations", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.config.AccessToken)
	req.Header.Set("Square-Version", "2024-10-17")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{resp.StatusCode}
	}
	var out struct {
		Locations []struct {
			ID           string   `json:"id"`
			Name         string   `json:"name"`
			Country      string   `json:"country"`
			Currency     string   `json:"currency"`
			Status       string   `json:"status"`
			Capabilities []string `json:"capabilities"`
		} `json:"locations"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	places := make([]Place, 0, len(out.Locations))
	for _, l := range out.Locations {
		pl := Place{ID: l.ID, Name: l.Name, Country: strings.ToUpper(l.Country), Currency: strings.ToLower(l.Currency),
			Status: l.Status, Capabilities: append([]string{}, l.Capabilities...), Methods: []string{}, Charging: l.ID == p.config.LocationID}
		if strings.EqualFold(l.Status, "ACTIVE") {
			for _, c := range l.Capabilities {
				if c == "CREDIT_CARD_PROCESSING" {
					pl.Methods = []string{"card", "apple_pay", "google_pay"}
					if pl.Country == "US" {
						pl.Methods = append(pl.Methods, "cash_app")
					}
				}
			}
		}
		places = append(places, pl)
	}
	return places, nil
}
