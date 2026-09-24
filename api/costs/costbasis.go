package costs

import "strings"

// costRate is what a provider is paid per million tokens, in cents — the COST
// side, deliberately DISTINCT from api/pricingrule (what customers are CHARGED).
// Keeping the two apart is the whole point: margin = charge - cost, so they must
// never share a table.
//
// Input and output are tracked separately and applied to the metered
// prompt/completion token split. The only source is the synced catalog
// (catalogbasis.go); a model it does not price contributes no estimated cost and
// is reported as unknown rather than guessed.
type costRate struct {
	// InputCentsPerMTok / OutputCentsPerMTok — cents paid per 1,000,000 tokens.
	InputCentsPerMTok  float64
	OutputCentsPerMTok float64
}

// modelCostCents applies a model's cost basis to a prompt/completion token split
// and returns the COGS in cents (rounded to the nearest cent). Returns (0,false)
// for an unknown model so the caller can report it honestly instead of guessing.
//
// Token counts are clamped to >= 0: a malformed usage row with a negative count
// would otherwise produce NEGATIVE COGS, understating cost and INFLATING margin (a
// dishonest figure). COGS never subtracts — a bad row contributes 0, not a credit.
func modelCostCents(catalog map[string]costRate, model string, promptTokens, completionTokens int64) (int64, bool) {
	rate, ok := basisRate(catalog, model)
	if !ok {
		return 0, false
	}
	if promptTokens < 0 {
		promptTokens = 0
	}
	if completionTokens < 0 {
		completionTokens = 0
	}
	cents := (float64(promptTokens)*rate.InputCentsPerMTok +
		float64(completionTokens)*rate.OutputCentsPerMTok) / 1_000_000
	return int64(cents + 0.5), true
}

// asInt64 coerces a JSON-decoded metadata value (float64 from encoding/json, or
// an int/int64/string) to int64. Metadata is stored as JSON, so numbers arrive as
// float64 — a plain `.(int64)` assertion would always miss. Returns 0 for a
// missing/unparseable value (a usage row with no token count contributes 0 COGS,
// which is correct — it was a zero-token call).
func asInt64(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	default:
		return 0
	}
}

// asString coerces a metadata value to a trimmed string, "" when absent.
func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}
