package model

import "testing"

// Catches grouping by profile instead of provider, merged provider aliases,
// profile mutation, and percentages computed against a single profile's cost.
func TestMultiProfileProviderTotals(t *testing.T) {
	work := &ProfileSummary{Profile: "work", Requests: 3, InputTokens: 30, OutputTokens: 10, CacheReadTokens: 5, Cost: 6, CostNoCache: 8,
		Providers: []*ProviderSubtotal{
			{Provider: "openai-codex", Requests: 2, InputTokens: 20, OutputTokens: 8, CacheReadTokens: 5, Cost: 4, CostNoCache: 6, SharePct: 66},
			{Provider: "openai", Requests: 1, InputTokens: 10, OutputTokens: 2, Cost: 2, CostNoCache: 2},
		}}
	personal := &ProfileSummary{Profile: "personal", Requests: 4, InputTokens: 40, OutputTokens: 15, CacheWriteTokens: 7, Cost: 4, CostNoCache: 5,
		Providers: []*ProviderSubtotal{
			{Provider: "openai-codex", Requests: 3, InputTokens: 25, OutputTokens: 10, CacheWriteTokens: 7, Cost: 3, CostNoCache: 4},
			{Provider: "anthropic", Requests: 1, InputTokens: 15, OutputTokens: 5, Cost: 1, CostNoCache: 1},
		}}
	r := NewMultiProfileReport("2026-10", []*ProfileSummary{work, personal, {Profile: "empty"}})
	if r.TotalCost != 10 || r.TotalRequests != 7 || r.TotalInput != 70 || r.TotalOutput != 25 || r.TotalCache != 12 || r.TotalCostNoCache != 13 {
		t.Fatalf("incorrect report totals: %+v", r)
	}
	if len(r.Providers) != 3 {
		t.Fatalf("expected 3 distinct providers, got %d", len(r.Providers))
	}
	p := r.Providers[0]
	if p.Provider != "openai-codex" || p.Cost != 7 || p.Requests != 5 || p.InputTokens != 45 || p.OutputTokens != 18 || p.CacheReadTokens != 5 || p.CacheWriteTokens != 7 || p.CostNoCache != 10 || p.SharePct != 70 {
		t.Fatalf("incorrect combined provider: %+v", p)
	}
	if r.Providers[1].Provider != "openai" || r.Providers[1].SharePct != 20 || r.Providers[2].Provider != "anthropic" || r.Providers[2].SharePct != 10 {
		t.Fatalf("incorrect unique provider totals: %+v", r.Providers)
	}
	if work.Providers[0].Cost != 4 || work.Providers[0].SharePct != 66 {
		t.Fatal("combined totals mutated per-profile subtotals")
	}
}

func TestMultiProfileZeroCostProviders(t *testing.T) {
	r := NewMultiProfileReport("2026-10", []*ProfileSummary{{Requests: 2, Providers: []*ProviderSubtotal{
		{Provider: "z-free", Requests: 1}, {Provider: "a-free", Requests: 1},
	}}})
	if r.Providers[0].Provider != "a-free" || r.Providers[1].Provider != "z-free" {
		t.Fatal("equal-cost providers must have stable alphabetical order")
	}
	for _, p := range r.Providers {
		if p.SharePct != 0 {
			t.Fatalf("zero spend must have zero share, got %v", p.SharePct)
		}
	}
}
