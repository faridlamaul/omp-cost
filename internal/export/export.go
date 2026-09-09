package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/faridlamaul/omp-cost/internal/model"
	"github.com/faridlamaul/omp-cost/internal/ui"
)

// ToJSON serializes any data structure into formatted JSON.
func ToJSON(data any) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ToCSV serializes profile models into CSV format.
func ToCSV(profiles []*model.ProfileSummary) (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)

	header := []string{
		"Month",
		"Profile",
		"Provider",
		"Model",
		"Requests",
		"InputTokens",
		"OutputTokens",
		"CacheTokens",
		"TotalCostUSD",
		"CostNoCacheUSD",
		"AvgDurationMs",
		"AvgTtftMs",
	}
	if err := w.Write(header); err != nil {
		return "", err
	}

	for _, p := range profiles {
		for _, prov := range p.Providers {
			for _, m := range prov.Models {
				row := []string{
					p.Month,
					p.Profile,
					m.Provider,
					m.Model,
					fmt.Sprintf("%d", m.Requests),
					fmt.Sprintf("%d", m.InputTokens),
					fmt.Sprintf("%d", m.OutputTokens),
					fmt.Sprintf("%d", m.CacheTokens()),
					fmt.Sprintf("%.4f", m.Cost),
					fmt.Sprintf("%.4f", m.CostNoCache),
					fmt.Sprintf("%.1f", m.AvgDurationMs),
					fmt.Sprintf("%.1f", m.AvgTtftMs),
				}
				if err := w.Write(row); err != nil {
					return "", err
				}
			}
		}
	}

	w.Flush()
	return b.String(), w.Error()
}

// ToDailyCSV serializes daily metrics into CSV format.
func ToDailyCSV(profiles []*model.ProfileSummary) (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)

	header := []string{"Date", "DayOfWeek", "Profile", "Requests", "InputTokens", "OutputTokens", "CacheTokens", "CostUSD"}
	if err := w.Write(header); err != nil {
		return "", err
	}

	for _, p := range profiles {
		for _, d := range p.Days {
			row := []string{
				d.Date,
				d.DayOfWeek,
				p.Profile,
				fmt.Sprintf("%d", d.Requests),
				fmt.Sprintf("%d", d.InputTokens),
				fmt.Sprintf("%d", d.OutputTokens),
				fmt.Sprintf("%d", d.CacheTokens()),
				fmt.Sprintf("%.4f", d.Cost),
			}
			if err := w.Write(row); err != nil {
				return "", err
			}
		}
	}

	w.Flush()
	return b.String(), w.Error()
}

// ToWeeklyCSV serializes weekly metrics into CSV format.
func ToWeeklyCSV(profiles []*model.ProfileSummary) (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)

	header := []string{"Week", "DateRange", "Profile", "Requests", "InputTokens", "OutputTokens", "CacheTokens", "CostUSD"}
	if err := w.Write(header); err != nil {
		return "", err
	}

	for _, p := range profiles {
		for _, wk := range p.Weeks {
			row := []string{
				wk.Week,
				wk.DateRange,
				p.Profile,
				fmt.Sprintf("%d", wk.Requests),
				fmt.Sprintf("%d", wk.InputTokens),
				fmt.Sprintf("%d", wk.OutputTokens),
				fmt.Sprintf("%d", wk.CacheTokens()),
				fmt.Sprintf("%.4f", wk.Cost),
			}
			if err := w.Write(row); err != nil {
				return "", err
			}
		}
	}

	w.Flush()
	return b.String(), w.Error()
}

// ToMarkdown serializes multi-profile or profile summaries into a Markdown table.
func ToMarkdown(report *model.MultiProfileReport) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# ⚡ Oh My Pi Usage & Cost Report — %s\n\n", report.Month))

	// Overview Section
	b.WriteString("## Profiles Summary\n\n")
	b.WriteString("| Profile | Requests | In/Out Tokens | Cache Tokens | Cost (USD) | Share |\n")
	b.WriteString("|:---|---:|---:|---:|---:|---:|\n")

	for _, p := range report.Profiles {
		cTokens := p.CacheTokens()
		share := 0.0
		if report.TotalCost > 0 {
			share = (p.Cost / report.TotalCost) * 100.0
		}
		inOut := fmt.Sprintf("%s/%s", ui.FormatTokens(p.InputTokens), ui.FormatTokens(p.OutputTokens))
		b.WriteString(fmt.Sprintf("| **%s** | %s | %s | %s | %s | %.1f%% |\n",
			p.Profile,
			formatInt(p.Requests),
			inOut,
			ui.FormatTokens(cTokens),
			ui.FormatCost(p.Cost),
			share,
		))
	}
	b.WriteString(fmt.Sprintf("| **TOTAL** | **%s** | **%s/%s** | **%s** | **%s** | **100.0%%** |\n\n",
		formatInt(report.TotalRequests),
		ui.FormatTokens(report.TotalInput),
		ui.FormatTokens(report.TotalOutput),
		ui.FormatTokens(report.TotalCache),
		ui.FormatCost(report.TotalCost),
	))

	// Detailed Model breakdown
	b.WriteString("## Detailed Model Breakdown\n\n")
	b.WriteString("| Profile | Provider | Model | Requests | In/Out Tokens | Cache Hit | Cost (USD) |\n")
	b.WriteString("|:---|:---|:---|---:|---:|---:|---:|\n")

	for _, p := range report.Profiles {
		for _, prov := range p.Providers {
			for _, m := range prov.Models {
				inOut := fmt.Sprintf("%s/%s", ui.FormatTokens(m.InputTokens), ui.FormatTokens(m.OutputTokens))
				b.WriteString(fmt.Sprintf("| **%s** | %s | `%s` | %s | %s | %.1f%% | %s |\n",
					p.Profile,
					m.Provider,
					m.Model,
					formatInt(m.Requests),
					inOut,
					m.CacheHitRate(),
					ui.FormatCost(m.Cost),
				))
			}
		}
	}

	// Daily Breakdown
	hasDays := false
	for _, p := range report.Profiles {
		if len(p.Days) > 0 {
			hasDays = true
			break
		}
	}
	if hasDays {
		b.WriteString("\n## Daily Breakdown\n\n")
		b.WriteString("| Date | Day | Profile | Requests | In/Out Tokens | Cache Hit | Cost (USD) |\n")
		b.WriteString("|:---|:---|:---|---:|---:|---:|---:|\n")
		for _, p := range report.Profiles {
			for _, d := range p.Days {
				inOut := fmt.Sprintf("%s/%s", ui.FormatTokens(d.InputTokens), ui.FormatTokens(d.OutputTokens))
				b.WriteString(fmt.Sprintf("| %s | %s | **%s** | %s | %s | %.1f%% | %s |\n",
					d.Date, d.DayOfWeek, p.Profile, formatInt(d.Requests), inOut, d.CacheHitRate(), ui.FormatCost(d.Cost)))
			}
		}
	}

	// Weekly Breakdown
	hasWeeks := false
	for _, p := range report.Profiles {
		if len(p.Weeks) > 0 {
			hasWeeks = true
			break
		}
	}
	if hasWeeks {
		b.WriteString("\n## Weekly Breakdown\n\n")
		b.WriteString("| Week | Date Range | Profile | Requests | In/Out Tokens | Cache Hit | Cost (USD) |\n")
		b.WriteString("|:---|:---|:---|---:|---:|---:|---:|\n")
		for _, p := range report.Profiles {
			for _, wk := range p.Weeks {
				inOut := fmt.Sprintf("%s/%s", ui.FormatTokens(wk.InputTokens), ui.FormatTokens(wk.OutputTokens))
				b.WriteString(fmt.Sprintf("| %s | %s | **%s** | %s | %s | %.1f%% | %s |\n",
					wk.Week, wk.DateRange, p.Profile, formatInt(wk.Requests), inOut, wk.CacheHitRate(), ui.FormatCost(wk.Cost)))
			}
		}
	}

	return b.String()
}

func formatInt(n int64) string {
	in := fmt.Sprintf("%d", n)
	var out []byte
	l := len(in)
	for i, c := range []byte(in) {
		if i > 0 && (l-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}
