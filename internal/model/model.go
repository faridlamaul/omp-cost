package model

import (
	"fmt"
	"time"
)

// ModelCost holds aggregated usage statistics for a specific provider and model.
type ModelCost struct {
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	Requests        int64   `json:"requests"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CacheReadTokens int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
	TotalTokens     int64   `json:"total_tokens"`
	Cost            float64 `json:"cost"`
	CostNoCache     float64 `json:"cost_no_cache"`
	AvgDurationMs   float64 `json:"avg_duration_ms"`
	AvgTtftMs       float64 `json:"avg_ttft_ms"`
	SharePct        float64 `json:"share_pct"`
}

func (m ModelCost) CacheTokens() int64 {
	return m.CacheReadTokens + m.CacheWriteTokens
}

func (m ModelCost) CacheHitRate() float64 {
	denom := m.CacheTokens() + m.InputTokens
	if denom == 0 {
		return 0.0
	}
	return (float64(m.CacheTokens()) / float64(denom)) * 100.0
}

func (m ModelCost) TokensPerSec() float64 {
	if m.AvgDurationMs <= 0 || m.OutputTokens == 0 || m.Requests == 0 {
		return 0.0
	}
	totalDurationSec := (m.AvgDurationMs * float64(m.Requests)) / 1000.0
	if totalDurationSec <= 0 {
		return 0.0
	}
	return float64(m.OutputTokens) / totalDurationSec
}

// ProviderSubtotal represents aggregated metrics for all models under one provider.
type ProviderSubtotal struct {
	Provider        string       `json:"provider"`
	Models          []*ModelCost `json:"models"`
	Requests        int64        `json:"requests"`
	InputTokens     int64        `json:"input_tokens"`
	OutputTokens    int64        `json:"output_tokens"`
	CacheReadTokens int64        `json:"cache_read_tokens"`
	CacheWriteTokens int64       `json:"cache_write_tokens"`
	Cost            float64      `json:"cost"`
	CostNoCache     float64      `json:"cost_no_cache"`
	SharePct        float64      `json:"share_pct"`
}

func (p ProviderSubtotal) CacheTokens() int64 {
	return p.CacheReadTokens + p.CacheWriteTokens
}

func (p ProviderSubtotal) CacheHitRate() float64 {
	denom := p.CacheTokens() + p.InputTokens
	if denom == 0 {
		return 0.0
	}
	return (float64(p.CacheTokens()) / float64(denom)) * 100.0
}

// ProjectCost holds attribution data per repository or workspace folder.
type ProjectCost struct {
	Folder          string  `json:"folder"`
	DisplayName     string  `json:"display_name"`
	Requests        int64   `json:"requests"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CacheTokens     int64   `json:"cache_tokens"`
	Cost            float64 `json:"cost"`
	TopModel        string  `json:"top_model"`
	SharePct        float64 `json:"share_pct"`
}

// AgentTypeCost holds attribution data for main orchestrator vs subagents vs advisor.
type AgentTypeCost struct {
	AgentType    string  `json:"agent_type"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CacheTokens  int64   `json:"cache_tokens"`
	Cost         float64 `json:"cost"`
	SharePct     float64 `json:"share_pct"`
}

// CacheSavings calculates the dollar value saved via prompt caching.
type CacheSavings struct {
	NetSpend        float64 `json:"net_spend"`
	EstimatedNoCache float64 `json:"estimated_no_cache"`
	DollarsSaved    float64 `json:"dollars_saved"`
	SavingsPct      float64 `json:"savings_pct"`
}

// BurnRateForecast calculates daily run-rate and month-end projection.
type BurnRateForecast struct {
	Month           string    `json:"month"`
	ActiveDays      int       `json:"active_days"`
	TotalDaysInMonth int      `json:"total_days_in_month"`
	CurrentSpend    float64   `json:"current_spend"`
	DailyBurnRate   float64   `json:"daily_burn_rate"`
	ProjectedSpend  float64   `json:"projected_spend"`
	BudgetCap       float64   `json:"budget_cap,omitempty"`
	OverBudget      bool      `json:"over_budget"`
}

// ProfileSummary represents high-level metrics for a profile in a given month.
type ProfileSummary struct {
	Profile         string              `json:"profile"`
	Month           string              `json:"month"`
	Requests        int64               `json:"requests"`
	InputTokens     int64               `json:"input_tokens"`
	OutputTokens    int64               `json:"output_tokens"`
	CacheReadTokens int64               `json:"cache_read_tokens"`
	CacheWriteTokens int64              `json:"cache_write_tokens"`
	TotalTokens     int64               `json:"total_tokens"`
	Cost            float64             `json:"cost"`
	CostNoCache     float64             `json:"cost_no_cache"`
	MinTimestamp    int64               `json:"min_timestamp"`
	MaxTimestamp    int64               `json:"max_timestamp"`
	Providers       []*ProviderSubtotal `json:"providers,omitempty"`
	Projects        []*ProjectCost      `json:"projects,omitempty"`
	AgentTypes      []*AgentTypeCost    `json:"agent_types,omitempty"`
	Savings         CacheSavings        `json:"savings"`
	Forecast        BurnRateForecast    `json:"forecast"`
}

func (s ProfileSummary) CacheTokens() int64 {
	return s.CacheReadTokens + s.CacheWriteTokens
}

func (s ProfileSummary) CacheHitRate() float64 {
	denom := s.CacheTokens() + s.InputTokens
	if denom == 0 {
		return 0.0
	}
	return (float64(s.CacheTokens()) / float64(denom)) * 100.0
}

func (s ProfileSummary) DateRangeString() string {
	if s.MinTimestamp <= 0 || s.MaxTimestamp <= 0 {
		return ""
	}
	tStart := time.UnixMilli(s.MinTimestamp).Local()
	tEnd := time.UnixMilli(s.MaxTimestamp).Local()
	return fmt.Sprintf("%s (%s – %s)", tStart.Format("January 2006"), tStart.Format("02 Jan"), tEnd.Format("02 Jan 2006"))
}

// MultiProfileReport combines summaries of multiple profiles.
type MultiProfileReport struct {
	Month           string            `json:"month"`
	Profiles        []*ProfileSummary `json:"profiles"`
	TotalRequests   int64             `json:"total_requests"`
	TotalInput      int64             `json:"total_input_tokens"`
	TotalOutput     int64             `json:"total_output_tokens"`
	TotalCache      int64             `json:"total_cache_tokens"`
	TotalCost       float64           `json:"total_cost"`
	TotalCostNoCache float64          `json:"total_cost_no_cache"`
	TotalSavings    CacheSavings      `json:"total_savings"`
	CombinedForecast BurnRateForecast `json:"combined_forecast"`
}

func (m MultiProfileReport) OverallCacheHitRate() float64 {
	denom := m.TotalCache + m.TotalInput
	if denom == 0 {
		return 0.0
	}
	return (float64(m.TotalCache) / float64(denom)) * 100.0
}
