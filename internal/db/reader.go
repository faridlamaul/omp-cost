package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/faridlamaul/omp-cost/internal/model"
	_ "modernc.org/sqlite"
)

// Reader handles querying SQLite stats.db instances.
type Reader struct{}

// NewReader creates a new Reader.
func NewReader() *Reader {
	return &Reader{}
}

// DiscoverProfiles scans ~/.omp for available profiles with stats.db files.
func (r *Reader) DiscoverProfiles() map[string]string {
	home, err := os.UserHomeDir()
	if err != nil {
		return map[string]string{}
	}

	profiles := make(map[string]string)
	resolvedPaths := make(map[string]bool)

	// 1. Default profile at ~/.omp/stats.db
	defDB := filepath.Join(home, ".omp", "stats.db")
	if fi, err := os.Stat(defDB); err == nil && !fi.IsDir() {
		realPath, err := filepath.EvalSymlinks(defDB)
		if err == nil {
			profiles["default"] = defDB
			resolvedPaths[realPath] = true
		}
	}

	// 2. Named profiles under ~/.omp/profiles/*/stats.db
	profilesDir := filepath.Join(home, ".omp", "profiles")
	entries, err := os.ReadDir(profilesDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && (entry.Type()&os.ModeSymlink == 0) {
				continue
			}
			dbCandidate := filepath.Join(profilesDir, entry.Name(), "stats.db")
			if fi, err := os.Stat(dbCandidate); err == nil && !fi.IsDir() {
				realPath, err := filepath.EvalSymlinks(dbCandidate)
				if err == nil {
					// Avoid adding duplicate real files under different alias names
					if !resolvedPaths[realPath] {
						profiles[entry.Name()] = dbCandidate
						resolvedPaths[realPath] = true
					}
				}
			}
		}
	}

	return profiles
}

// SyncProfilesConcurrently runs 'omp stats -s' concurrently across profiles.
func (r *Reader) SyncProfilesConcurrently(profiles []string) {
	var wg sync.WaitGroup
	for _, p := range profiles {
		wg.Add(1)
		go func(profile string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()

			args := []string{}
			if profile != "default" && profile != "all" {
				args = append(args, "--profile", profile)
			}
			args = append(args, "stats", "-s")

			cmd := exec.CommandContext(ctx, "omp", args...)
			_ = cmd.Run()
		}(p)
	}
	wg.Wait()
}

// OpenDB opens a connection to a stats.db with read-only pragmas.
func (r *Reader) OpenDB(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", filepath.ToSlash(path))
	return sql.Open("sqlite", dsn)
}

// GetDistinctMonths returns all available YYYY-MM months in descending order.
func (r *Reader) GetDistinctMonths(dbPath string) ([]string, error) {
	conn, err := r.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	query := `
	SELECT DISTINCT strftime('%Y-%m', timestamp/1000, 'unixepoch', 'localtime') as m
	FROM messages
	WHERE timestamp IS NOT NULL
	ORDER BY m DESC;
	`
	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var months []string
	for rows.Next() {
		var m sql.NullString
		if err := rows.Scan(&m); err == nil && m.Valid && m.String != "" {
			months = append(months, m.String)
		}
	}
	return months, nil
}

// GetModelCosts queries aggregated provider/model metrics for a given month.
func (r *Reader) GetModelCosts(dbPath string, month string) ([]*model.ModelCost, int64, int64, error) {
	conn, err := r.OpenDB(dbPath)
	if err != nil {
		return nil, 0, 0, err
	}
	defer conn.Close()

	whereClause := ""
	var args []interface{}
	if month != "" && month != "all" {
		if len(month) == 10 { // YYYY-MM-DD
			whereClause = "WHERE strftime('%Y-%m-%d', timestamp/1000, 'unixepoch', 'localtime') = ?"
		} else { // YYYY-MM
			whereClause = "WHERE strftime('%Y-%m', timestamp/1000, 'unixepoch', 'localtime') = ?"
		}
		args = append(args, month)
	}

	query := fmt.Sprintf(`
	SELECT 
		provider,
		model,
		count(*) as reqs,
		coalesce(sum(input_tokens), 0) as in_tok,
		coalesce(sum(output_tokens), 0) as out_tok,
		coalesce(sum(cache_read_tokens), 0) as cache_read_tok,
		coalesce(sum(cache_write_tokens), 0) as cache_write_tok,
		coalesce(sum(total_tokens), 0) as total_tok,
		coalesce(sum(cost_total), 0) as cost,
		coalesce(sum(cost_no_cache_input), 0) as cost_no_cache,
		coalesce(avg(duration), 0) as avg_duration,
		coalesce(avg(ttft), 0) as avg_ttft,
		coalesce(min(timestamp), 0) as min_ts,
		coalesce(max(timestamp), 0) as max_ts
	FROM messages
	%s
	GROUP BY provider, model
	ORDER BY cost DESC;
	`, whereClause)

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	var results []*model.ModelCost
	var minTsAll int64 = 0
	var maxTsAll int64 = 0

	for rows.Next() {
		m := &model.ModelCost{}
		var minTs, maxTs int64
		err := rows.Scan(
			&m.Provider,
			&m.Model,
			&m.Requests,
			&m.InputTokens,
			&m.OutputTokens,
			&m.CacheReadTokens,
			&m.CacheWriteTokens,
			&m.TotalTokens,
			&m.Cost,
			&m.CostNoCache,
			&m.AvgDurationMs,
			&m.AvgTtftMs,
			&minTs,
			&maxTs,
		)
		if err != nil {
			continue
		}
		if minTsAll == 0 || (minTs > 0 && minTs < minTsAll) {
			minTsAll = minTs
		}
		if maxTs > maxTsAll {
			maxTsAll = maxTs
		}
		results = append(results, m)
	}

	return results, minTsAll, maxTsAll, nil
}

// GetProjectCosts queries cost breakdown per repository or workspace folder.
func (r *Reader) GetProjectCosts(dbPath string, month string) ([]*model.ProjectCost, error) {
	conn, err := r.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	whereClause := ""
	var args []interface{}
	if month != "" && month != "all" {
		whereClause = "WHERE strftime('%Y-%m', timestamp/1000, 'unixepoch', 'localtime') = ?"
		args = append(args, month)
	}

	query := fmt.Sprintf(`
	SELECT 
		folder,
		count(*) as reqs,
		coalesce(sum(input_tokens), 0) as in_tok,
		coalesce(sum(output_tokens), 0) as out_tok,
		coalesce(sum(cache_read_tokens + cache_write_tokens), 0) as cache_tok,
		coalesce(sum(cost_total), 0) as cost
	FROM messages
	%s
	GROUP BY folder
	ORDER BY cost DESC;
	`, whereClause)

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*model.ProjectCost
	for rows.Next() {
		p := &model.ProjectCost{}
		var folder sql.NullString
		if err := rows.Scan(&folder, &p.Requests, &p.InputTokens, &p.OutputTokens, &p.CacheTokens, &p.Cost); err != nil {
			continue
		}
		if folder.Valid {
			p.Folder = folder.String
			p.DisplayName = formatFolderName(folder.String)
		} else {
			p.Folder = "unknown"
			p.DisplayName = "unknown"
		}
		projects = append(projects, p)
	}

	return projects, nil
}

// GetAgentTypeCosts queries cost breakdown per agent architecture type (main, subagent, advisor).
func (r *Reader) GetAgentTypeCosts(dbPath string, month string) ([]*model.AgentTypeCost, error) {
	conn, err := r.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	whereClause := ""
	var args []interface{}
	if month != "" && month != "all" {
		whereClause = "WHERE strftime('%Y-%m', timestamp/1000, 'unixepoch', 'localtime') = ?"
		args = append(args, month)
	}

	query := fmt.Sprintf(`
	SELECT 
		coalesce(nullif(agent_type, ''), 'main') as a_type,
		count(*) as reqs,
		coalesce(sum(input_tokens), 0) as in_tok,
		coalesce(sum(output_tokens), 0) as out_tok,
		coalesce(sum(cache_read_tokens + cache_write_tokens), 0) as cache_tok,
		coalesce(sum(cost_total), 0) as cost
	FROM messages
	%s
	GROUP BY a_type
	ORDER BY cost DESC;
	`, whereClause)

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agentTypes []*model.AgentTypeCost
	for rows.Next() {
		a := &model.AgentTypeCost{}
		if err := rows.Scan(&a.AgentType, &a.Requests, &a.InputTokens, &a.OutputTokens, &a.CacheTokens, &a.Cost); err != nil {
			continue
		}
		agentTypes = append(agentTypes, a)
	}

	return agentTypes, nil
}

// GetDailyCosts queries aggregated metrics per calendar day.
func (r *Reader) GetDailyCosts(dbPath string, month string) ([]*model.DailyCost, error) {
	conn, err := r.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	whereClause := ""
	var args []interface{}
	if month != "" && month != "all" {
		whereClause = "WHERE strftime('%Y-%m', timestamp/1000, 'unixepoch', 'localtime') = ?"
		args = append(args, month)
	}

	query := fmt.Sprintf(`
	SELECT 
		strftime('%%Y-%%m-%%d', timestamp/1000, 'unixepoch', 'localtime') as day,
		count(*) as reqs,
		coalesce(sum(input_tokens), 0) as in_tok,
		coalesce(sum(output_tokens), 0) as out_tok,
		coalesce(sum(cache_read_tokens), 0) as cache_read_tok,
		coalesce(sum(cache_write_tokens), 0) as cache_write_tok,
		coalesce(sum(cost_total), 0) as cost,
		coalesce(sum(cost_no_cache_input), 0) as cost_no_cache
	FROM messages
	%s
	GROUP BY day
	ORDER BY day DESC;
	`, whereClause)

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []*model.DailyCost
	for rows.Next() {
		d := &model.DailyCost{}
		if err := rows.Scan(&d.Date, &d.Requests, &d.InputTokens, &d.OutputTokens, &d.CacheReadTokens, &d.CacheWriteTokens, &d.Cost, &d.CostNoCache); err != nil {
			continue
		}
		if t, err := time.Parse("2006-01-02", d.Date); err == nil {
			d.DayOfWeek = t.Format("Mon")
		}
		days = append(days, d)
	}
	return days, nil
}

// GetWeeklyCosts queries aggregated metrics per calendar week.
func (r *Reader) GetWeeklyCosts(dbPath string, month string) ([]*model.WeeklyCost, error) {
	conn, err := r.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	whereClause := ""
	var args []interface{}
	if month != "" && month != "all" {
		whereClause = "WHERE strftime('%Y-%m', timestamp/1000, 'unixepoch', 'localtime') = ?"
		args = append(args, month)
	}

	query := fmt.Sprintf(`
	SELECT 
		strftime('%%Y-W%%W', timestamp/1000, 'unixepoch', 'localtime') as week,
		min(timestamp) as min_ts,
		max(timestamp) as max_ts,
		count(*) as reqs,
		coalesce(sum(input_tokens), 0) as in_tok,
		coalesce(sum(output_tokens), 0) as out_tok,
		coalesce(sum(cache_read_tokens), 0) as cache_read_tok,
		coalesce(sum(cache_write_tokens), 0) as cache_write_tok,
		coalesce(sum(cost_total), 0) as cost,
		coalesce(sum(cost_no_cache_input), 0) as cost_no_cache
	FROM messages
	%s
	GROUP BY week
	ORDER BY week DESC;
	`, whereClause)

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var weeks []*model.WeeklyCost
	for rows.Next() {
		w := &model.WeeklyCost{}
		var minTs, maxTs int64
		if err := rows.Scan(&w.Week, &minTs, &maxTs, &w.Requests, &w.InputTokens, &w.OutputTokens, &w.CacheReadTokens, &w.CacheWriteTokens, &w.Cost, &w.CostNoCache); err != nil {
			continue
		}
		if minTs > 0 && maxTs > 0 {
			tStart := time.UnixMilli(minTs).Local()
			tEnd := time.UnixMilli(maxTs).Local()
			w.DateRange = fmt.Sprintf("%s – %s", tStart.Format("02 Jan"), tEnd.Format("02 Jan"))
		}
		weeks = append(weeks, w)
	}
	return weeks, nil
}

// formatFolderName cleans OMP folder names like "-Workspace-myproject-api" into cleaner paths.
func formatFolderName(folder string) string {
	if folder == "-tmp" || folder == "tmp" {
		return "tmp"
	}
	cleaned := strings.TrimPrefix(folder, "-")
	cleaned = strings.ReplaceAll(cleaned, "-", "/")
	parts := strings.Split(cleaned, "/")
	if len(parts) > 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return folder
}

// BuildProfileSummary compiles all metrics into a complete ProfileSummary struct.
func (r *Reader) BuildProfileSummary(profileName, dbPath, month string, budgetCap float64) (*model.ProfileSummary, error) {
	modelCosts, minTs, maxTs, err := r.GetModelCosts(dbPath, month)
	if err != nil {
		return nil, err
	}
	if len(modelCosts) == 0 {
		return &model.ProfileSummary{
			Profile: profileName,
			Month:   month,
		}, nil
	}

	summary := &model.ProfileSummary{
		Profile:      profileName,
		Month:        month,
		MinTimestamp: minTs,
		MaxTimestamp: maxTs,
	}

	// Group models by provider
	providerMap := make(map[string]*model.ProviderSubtotal)
	var grandCost float64 = 0.0

	for _, mc := range modelCosts {
		summary.Requests += mc.Requests
		summary.InputTokens += mc.InputTokens
		summary.OutputTokens += mc.OutputTokens
		summary.CacheReadTokens += mc.CacheReadTokens
		summary.CacheWriteTokens += mc.CacheWriteTokens
		summary.TotalTokens += mc.TotalTokens
		summary.Cost += mc.Cost
		summary.CostNoCache += mc.CostNoCache
		grandCost += mc.Cost

		sub, exists := providerMap[mc.Provider]
		if !exists {
			sub = &model.ProviderSubtotal{
				Provider: mc.Provider,
			}
			providerMap[mc.Provider] = sub
		}
		sub.Models = append(sub.Models, mc)
		sub.Requests += mc.Requests
		sub.InputTokens += mc.InputTokens
		sub.OutputTokens += mc.OutputTokens
		sub.CacheReadTokens += mc.CacheReadTokens
		sub.CacheWriteTokens += mc.CacheWriteTokens
		sub.Cost += mc.Cost
		sub.CostNoCache += mc.CostNoCache
	}

	// Calculate share percentages for models and providers
	var providers []*model.ProviderSubtotal
	for _, sub := range providerMap {
		if grandCost > 0 {
			sub.SharePct = (sub.Cost / grandCost) * 100.0
		}
		for _, m := range sub.Models {
			if grandCost > 0 {
				m.SharePct = (m.Cost / grandCost) * 100.0
			}
		}
		// Sort models in provider by cost descending
		sort.Slice(sub.Models, func(i, j int) bool {
			return sub.Models[i].Cost > sub.Models[j].Cost
		})
		providers = append(providers, sub)
	}

	// Sort providers by cost descending
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Cost > providers[j].Cost
	})
	summary.Providers = providers

	// Projects
	projects, _ := r.GetProjectCosts(dbPath, month)
	for _, p := range projects {
		if grandCost > 0 {
			p.SharePct = (p.Cost / grandCost) * 100.0
		}
	}
	summary.Projects = projects

	// Agent Types
	agentTypes, _ := r.GetAgentTypeCosts(dbPath, month)
	for _, a := range agentTypes {
		if grandCost > 0 {
			a.SharePct = (a.Cost / grandCost) * 100.0
		}
	}
	summary.AgentTypes = agentTypes

	// Daily Costs
	days, _ := r.GetDailyCosts(dbPath, month)
	for _, d := range days {
		if grandCost > 0 {
			d.SharePct = (d.Cost / grandCost) * 100.0
		}
	}
	summary.Days = days

	// Weekly Costs
	weeks, _ := r.GetWeeklyCosts(dbPath, month)
	for _, w := range weeks {
		if grandCost > 0 {
			w.SharePct = (w.Cost / grandCost) * 100.0
		}
	}
	summary.Weeks = weeks

	// Cache Savings Calculation
	estimatedNoCache := summary.CostNoCache
	if estimatedNoCache < summary.Cost {
		// Fallback: estimate based on standard input token cost multiplier if cost_no_cache wasn't populated
		estimatedNoCache = summary.Cost + (float64(summary.CacheTokens()) * 0.000003)
	}
	dollarsSaved := estimatedNoCache - summary.Cost
	if dollarsSaved < 0 {
		dollarsSaved = 0
	}
	savingsPct := 0.0
	if estimatedNoCache > 0 {
		savingsPct = (dollarsSaved / estimatedNoCache) * 100.0
	}
	summary.Savings = model.CacheSavings{
		NetSpend:         summary.Cost,
		EstimatedNoCache: estimatedNoCache,
		DollarsSaved:     dollarsSaved,
		SavingsPct:       savingsPct,
	}

	// Burn Rate Forecast
	summary.Forecast = calculateForecast(month, minTs, maxTs, summary.Cost, budgetCap)

	return summary, nil
}

// calculateForecast computes burn-rate and end-of-month projection.
func calculateForecast(month string, minTs, maxTs int64, currentSpend, budgetCap float64) model.BurnRateForecast {
	now := time.Now()
	totalDays := 30
	activeDays := 1

	if t, err := time.Parse("2006-01", month); err == nil {
		// Calculate days in that month
		nextM := t.AddDate(0, 1, 0)
		totalDays = int(nextM.Sub(t).Hours() / 24)

		if month == now.Format("2006-01") {
			activeDays = now.Day()
		} else {
			activeDays = totalDays
		}
	}

	if activeDays < 1 {
		activeDays = 1
	}

	dailyBurn := currentSpend / float64(activeDays)
	projected := dailyBurn * float64(totalDays)

	return model.BurnRateForecast{
		Month:            month,
		ActiveDays:       activeDays,
		TotalDaysInMonth: totalDays,
		CurrentSpend:     currentSpend,
		DailyBurnRate:    dailyBurn,
		ProjectedSpend:   projected,
		BudgetCap:        budgetCap,
		OverBudget:       budgetCap > 0 && projected > budgetCap,
	}
}
