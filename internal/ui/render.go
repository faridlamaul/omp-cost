package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/faridlamaul/omp-cost/internal/model"
)

// RenderFlags controls which additional analytical views to render.
type RenderFlags struct {
	ShowProjects bool
	ShowAgents   bool
	ShowPerf     bool
	ShowSavings  bool
	ShowForecast bool
	ShowDaily    bool
	ShowWeekly   bool
	BudgetCap    float64
}

const StandardTableWidth = 117

// GetTerminalWidth returns the standard table width.
func GetTerminalWidth() int {
	return StandardTableWidth
}

// RenderMultiProfileReport displays the combined overview across all profiles.
func RenderMultiProfileReport(report *model.MultiProfileReport, flags RenderFlags) {
	w := StandardTableWidth

	// 1. Header Card
	title := StyleHeaderTitle.Render("⚡ OH MY PI") + " " + StyleDim.Render("— Multi-Profile Cost & Usage Report")
	meta := fmt.Sprintf("%s %s  •  %s %s",
		StyleSecondary.Render("Profiles:"),
		lipgloss.NewStyle().Foreground(ColorSky).Render(fmt.Sprintf("All (%d)", len(report.Profiles))),
		StyleSecondary.Render("Period:"),
		lipgloss.NewStyle().Foreground(ColorYellow).Render(report.DateRangeString()),
	)

	headerCard := StyleCard.Width(w - 2).Render(fmt.Sprintf("%s\n%s", title, meta))
	fmt.Println(headerCard)
	// 2. KPI Cards
	renderKPICards(
		[]KPIItem{
			{Title: "COMBINED TOTAL COST", Value: FormatCost(report.TotalCost), Style: StyleCost},
			{Title: "TOTAL REQUESTS", Value: fmt.Sprintf("%s reqs", formatInt(report.TotalRequests)), Style: StyleBold},
			{Title: "OVERALL CACHE HIT", Value: fmt.Sprintf("%.1f%% (%s)", report.OverallCacheHitRate(), FormatTokens(report.TotalCache)), Style: lipgloss.NewStyle().Bold(true).Foreground(ColorSky)},
		},
		w,
	)

	// 3. Optional Savings & Forecast Banners
	if flags.ShowSavings || report.TotalSavings.DollarsSaved > 0 {
		RenderSavingsBanner(report.TotalSavings, w)
	}
	if flags.ShowForecast || flags.BudgetCap > 0 {
		RenderForecastBanner(report.CombinedForecast, w)
	}

	// 4. Profiles Overview Table
	fmt.Println(renderProfilesSummaryTable(report.Profiles, report.TotalCost, w))

	// 5. Active Profile Details
	for _, p := range report.Profiles {
		if p.Requests > 0 {
			RenderProfileSummary(p, flags, true)
		}
	}
}

// RenderProfileSummary renders an individual profile's detailed report.
func RenderProfileSummary(summary *model.ProfileSummary, flags RenderFlags, isSubView bool) {
	w := StandardTableWidth

	if !isSubView {
		title := StyleHeaderTitle.Render("⚡ OH MY PI") + " " + StyleDim.Render(fmt.Sprintf("— Profile Detail: %s", summary.Profile))
		dateStr := summary.DateRangeString()
		if dateStr == "" {
			dateStr = summary.Month
		}
		meta := fmt.Sprintf("%s %s", StyleSecondary.Render("Period:"), lipgloss.NewStyle().Foreground(ColorYellow).Render(dateStr))

		headerCard := StyleCard.Width(w - 2).Render(fmt.Sprintf("%s\n%s", title, meta))
		fmt.Println(headerCard)
		renderKPICards(
			[]KPIItem{
				{Title: "TOTAL COST", Value: FormatCost(summary.Cost), Style: StyleCost},
				{Title: "TOTAL REQUESTS", Value: fmt.Sprintf("%s reqs", formatInt(summary.Requests)), Style: StyleBold},
				{Title: "CACHE EFFICIENCY", Value: fmt.Sprintf("%.1f%% (%s)", summary.CacheHitRate(), FormatTokens(summary.CacheTokens())), Style: lipgloss.NewStyle().Bold(true).Foreground(ColorSky)},
			},
			w,
		)

		if flags.ShowSavings || summary.Savings.DollarsSaved > 0 {
			RenderSavingsBanner(summary.Savings, w)
		}
		if flags.ShowForecast || flags.BudgetCap > 0 {
			RenderForecastBanner(summary.Forecast, w)
		}
	} else {
		badge := lipgloss.NewStyle().Bold(true).Foreground(ColorMauve).Render(fmt.Sprintf("▶ Profile: %s", summary.Profile))
		spend := StyleCost.Render(FormatCost(summary.Cost))
		reqs := StyleDim.Render(fmt.Sprintf("(%s reqs)", formatInt(summary.Requests)))
		fmt.Printf("\n%s %s %s\n", badge, spend, reqs)
	}

	// Models & Provider Table
	fmt.Println(renderModelBreakdownTable(summary, w))

	// Optional Sections
	if flags.ShowProjects && len(summary.Projects) > 0 {
		RenderProjectAttributionTable(summary.Projects, summary.Cost, w)
	}
	if flags.ShowAgents && len(summary.AgentTypes) > 0 {
		RenderAgentAttributionTable(summary.AgentTypes, summary.Cost, w)
	}
	if flags.ShowPerf {
		var allModels []*model.ModelCost
		for _, p := range summary.Providers {
			allModels = append(allModels, p.Models...)
		}
		RenderPerformanceBenchmarkTable(allModels, w)
	}
	if flags.ShowDaily && len(summary.Days) > 0 {
		RenderDailyBreakdownTable(summary.Days, summary.Cost, w)
	}
	if flags.ShowWeekly && len(summary.Weeks) > 0 {
		RenderWeeklyBreakdownTable(summary.Weeks, summary.Cost, w)
	}
}

// KPIItem represents a metric box in the top row.
type KPIItem struct {
	Title string
	Value string
	Style lipgloss.Style
}

func renderKPICards(items []KPIItem, totalW int) {
	if len(items) == 0 {
		return
	}
	cardW1 := totalW / 3
	cardW2 := totalW / 3
	cardW3 := totalW - cardW1 - cardW2
	cardWidths := []int{cardW1, cardW2, cardW3}

	var rendered []string
	for i, item := range items {
		titleText := StyleDim.Render(item.Title)
		valText := item.Style.Render(item.Value)
		card := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorLine).
			Padding(0, 1).
			Width(cardWidths[i] - 2).
			Render(fmt.Sprintf("%s\n%s", titleText, valText))
		rendered = append(rendered, card)
	}

	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top, rendered...))
	fmt.Println()
}

// RenderSavingsBanner displays prompt cache ROI in dollars and percentage.
func RenderSavingsBanner(savings model.CacheSavings, width int) {
	text := fmt.Sprintf("💰 %s %s  •  🛡️ %s %s  •  🎉 %s %s %s",
		StyleSecondary.Render("Net Spend:"),
		StyleCost.Render(FormatCost(savings.NetSpend)),
		StyleSecondary.Render("Without Cache:"),
		lipgloss.NewStyle().Foreground(ColorYellow).Render(FormatCost(savings.EstimatedNoCache)),
		StyleSecondary.Render("Dollar Saved:"),
		lipgloss.NewStyle().Bold(true).Foreground(ColorSky).Render(FormatCost(savings.DollarsSaved)),
		lipgloss.NewStyle().Foreground(ColorGreen).Render(fmt.Sprintf("(%.1f%% discount)", savings.SavingsPct)),
	)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorGreen).
		Width(width - 2).
		Padding(0, 1).
		Render(text)
	fmt.Println(box)
	fmt.Println()
}

// RenderForecastBanner displays daily burn-rate and month-end projection.
func RenderForecastBanner(f model.BurnRateForecast, width int) {
	statusColor := ColorSky
	statusText := ""
	if f.OverBudget {
		statusColor = ColorRed
		statusText = lipgloss.NewStyle().Bold(true).Foreground(ColorRed).Render(fmt.Sprintf(" [OVER BUDGET CAP %s]", FormatCost(f.BudgetCap)))
	} else if f.BudgetCap > 0 {
		statusText = lipgloss.NewStyle().Foreground(ColorGreen).Render(fmt.Sprintf(" [Under Budget %s]", FormatCost(f.BudgetCap)))
	}

	text := fmt.Sprintf("🔥 %s %s/day (day %d/%d)  •  📈 %s %s%s",
		StyleSecondary.Render("Daily Burn Rate:"),
		lipgloss.NewStyle().Bold(true).Foreground(ColorYellow).Render(FormatCost(f.DailyBurnRate)),
		f.ActiveDays, f.TotalDaysInMonth,
		StyleSecondary.Render("Projected Month-End:"),
		lipgloss.NewStyle().Bold(true).Foreground(statusColor).Render(FormatCost(f.ProjectedSpend)),
		statusText,
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(statusColor).
		Width(width - 2).
		Padding(0, 1).
		Render(text)
	fmt.Println(box)
	fmt.Println()
}

func renderProfilesSummaryTable(profiles []*model.ProfileSummary, grandTotal float64, totalW int) string {
	var b strings.Builder

	cols := []struct {
		name  string
		width int
		align lipgloss.Position
	}{
		{"Profile", 24, lipgloss.Left},
		{"Requests", 11, lipgloss.Right},
		{"In / Out Tokens", 18, lipgloss.Right},
		{"Cache Hit", 13, lipgloss.Right},
		{"Cost ($)", 14, lipgloss.Right},
		{"Share", 18, lipgloss.Right},
	}

	// Table borders
	b.WriteString(renderTableBorder("┌", "┬", "┐", cols))
	b.WriteString("\n")

	// Header row
	var headerCells []string
	for _, c := range cols {
		cell := lipgloss.NewStyle().Width(c.width).Align(c.align).Render(StyleTableHeader.Render(c.name))
		headerCells = append(headerCells, cell)
	}
	b.WriteString(renderTableRow(headerCells))
	b.WriteString("\n")
	b.WriteString(renderTableBorder("├", "┼", "┤", cols))
	b.WriteString("\n")

	// Data rows
	var totalReqs int64
	var totalIn, totalOut, totalCache int64

	for _, p := range profiles {
		totalReqs += p.Requests
		totalIn += p.InputTokens
		totalOut += p.OutputTokens
		totalCache += p.CacheTokens()

		cHit := "-"
		inOut := "-"
		shareStr := "-"
		bar := ""

		if p.Requests > 0 {
			cHit = fmt.Sprintf("%.1f%%", p.CacheHitRate())
			inOut = fmt.Sprintf("%s/%s", FormatTokens(p.InputTokens), FormatTokens(p.OutputTokens))
			share := 0.0
			if grandTotal > 0 {
				share = (p.Cost / grandTotal) * 100.0
			}
			bar = MakeProgressBar(share, 6)
			shareStr = fmt.Sprintf("%s %4.1f%%", bar, share)
		}

		cHitStyle := StyleSecondary
		if p.CacheHitRate() >= 90.0 && p.Requests > 0 {
			cHitStyle = lipgloss.NewStyle().Foreground(ColorGreen)
		}

		cells := []string{
			lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(lipgloss.NewStyle().Bold(true).Foreground(ColorSky).Render(p.Profile)),
			lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(formatInt(p.Requests)),
			lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(StyleSecondary.Render(inOut)),
			lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(cHitStyle.Render(cHit)),
			lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(StyleCost.Render(FormatCost(p.Cost))),
			lipgloss.NewStyle().Width(cols[5].width).Align(cols[5].align).Render(shareStr),
		}
		b.WriteString(renderTableRow(cells))
		b.WriteString("\n")
	}

	// Grand Total Row
	overallHit := 0.0
	if (totalCache + totalIn) > 0 {
		overallHit = (float64(totalCache) / float64(totalCache+totalIn)) * 100.0
	}
	totCells := []string{
		lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(StyleGrandTotal.Render("★ ALL PROFILES")),
		lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(StyleBold.Render(formatInt(totalReqs))),
		lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(StyleBold.Render(fmt.Sprintf("%s/%s", FormatTokens(totalIn), FormatTokens(totalOut)))),
		lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(StyleBold.Render(fmt.Sprintf("%.1f%%", overallHit))),
		lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(StyleCost.Render(FormatCost(grandTotal))),
		lipgloss.NewStyle().Width(cols[5].width).Align(cols[5].align).Render(fmt.Sprintf("%s 100.0%%", MakeProgressBar(100.0, 6))),
	}
	b.WriteString(renderTableBorder("╞", "╪", "╡", cols))
	b.WriteString("\n")
	b.WriteString(renderTableRow(totCells))
	b.WriteString("\n")
	b.WriteString(renderTableBorder("└", "┴", "┘", cols))

	return b.String()
}

func renderModelBreakdownTable(summary *model.ProfileSummary, totalW int) string {
	var b strings.Builder

	cols := []struct {
		name  string
		width int
		align lipgloss.Position
	}{
		{"Provider", 18, lipgloss.Left},
		{"Model", 20, lipgloss.Left},
		{"Reqs", 7, lipgloss.Right},
		{"In / Out", 13, lipgloss.Right},
		{"Cache Hit", 10, lipgloss.Right},
		{"Cost ($)", 11, lipgloss.Right},
		{"Share", 16, lipgloss.Right},
	}

	b.WriteString(renderTableBorder("┌", "┬", "┐", cols))
	b.WriteString("\n")

	var headerCells []string
	for _, c := range cols {
		cell := lipgloss.NewStyle().Width(c.width).Align(c.align).Render(StyleTableHeader.Render(c.name))
		headerCells = append(headerCells, cell)
	}
	b.WriteString(renderTableRow(headerCells))
	b.WriteString("\n")
	b.WriteString(renderTableBorder("├", "┼", "┤", cols))
	b.WriteString("\n")

	for idx, prov := range summary.Providers {
		first := true
		for _, m := range prov.Models {
			provName := ""
			if first {
				provName = StyleProvider.Render(truncate(prov.Provider, cols[0].width))
				first = false
			}

			cRate := m.CacheHitRate()
			cStyle := StyleSecondary
			if cRate >= 90.0 {
				cStyle = lipgloss.NewStyle().Foreground(ColorGreen)
			} else if cRate > 0 {
				cStyle = lipgloss.NewStyle().Foreground(ColorYellow)
			}

			bar := MakeProgressBar(m.SharePct, 6)
			shareStr := fmt.Sprintf("%s %4.1f%%", bar, m.SharePct)

			cells := []string{
				lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(provName),
				lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(StyleModel.Render(truncate(m.Model, cols[1].width))),
				lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(formatInt(m.Requests)),
				lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(StyleSecondary.Render(fmt.Sprintf("%s/%s", FormatTokens(m.InputTokens), FormatTokens(m.OutputTokens)))),
				lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(cStyle.Render(fmt.Sprintf("%.1f%%", cRate))),
				lipgloss.NewStyle().Width(cols[5].width).Align(cols[5].align).Render(StyleBold.Render(FormatCost(m.Cost))),
				lipgloss.NewStyle().Width(cols[6].width).Align(cols[6].align).Render(shareStr),
			}
			b.WriteString(renderTableRow(cells))
			b.WriteString("\n")
		}

		// Subtotal row
		subRate := prov.CacheHitRate()
		subBar := MakeProgressBar(prov.SharePct, 6)
		subCells := []string{
			lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(StyleDim.Render(truncate(prov.Provider, cols[0].width))),
			lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(StyleDim.Render("❯ Subtotal")),
			lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(StyleDim.Render(formatInt(prov.Requests))),
			lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(StyleDim.Render(fmt.Sprintf("%s/%s", FormatTokens(prov.InputTokens), FormatTokens(prov.OutputTokens)))),
			lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(StyleDim.Render(fmt.Sprintf("%.1f%%", subRate))),
			lipgloss.NewStyle().Width(cols[5].width).Align(cols[5].align).Render(StyleSubtotal.Render(FormatCost(prov.Cost))),
			lipgloss.NewStyle().Width(cols[6].width).Align(cols[6].align).Render(fmt.Sprintf("%s %s", subBar, StyleSubtotal.Render(fmt.Sprintf("%4.1f%%", prov.SharePct)))),
		}
		b.WriteString(renderTableBorder("├", "┼", "┤", cols))
		b.WriteString("\n")
		b.WriteString(renderTableRow(subCells))
		b.WriteString("\n")

		if idx < len(summary.Providers)-1 {
			b.WriteString(renderTableBorder("├", "┼", "┤", cols))
			b.WriteString("\n")
		}
	}

	// Total Row
	totHit := summary.CacheHitRate()
	totCells := []string{
		lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(StyleGrandTotal.Render("★ TOTAL")),
		lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(StyleBold.Render(fmt.Sprintf("ALL MODELS (%d)", countModels(summary)))),
		lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(StyleBold.Render(formatInt(summary.Requests))),
		lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(StyleBold.Render(fmt.Sprintf("%s/%s", FormatTokens(summary.InputTokens), FormatTokens(summary.OutputTokens)))),
		lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(StyleBold.Render(fmt.Sprintf("%.1f%%", totHit))),
		lipgloss.NewStyle().Width(cols[5].width).Align(cols[5].align).Render(StyleCost.Render(FormatCost(summary.Cost))),
		lipgloss.NewStyle().Width(cols[6].width).Align(cols[6].align).Render(fmt.Sprintf("%s %s", MakeProgressBar(100.0, 6), StyleCost.Render("100.0%"))),
	}

	b.WriteString(renderTableBorder("╞", "╪", "╡", cols))
	b.WriteString("\n")
	b.WriteString(renderTableRow(totCells))
	b.WriteString("\n")
	b.WriteString(renderTableBorder("└", "┴", "┘", cols))

	return b.String()
}

// RenderProjectAttributionTable renders cost broken down by workspace/repository.
func RenderProjectAttributionTable(projects []*model.ProjectCost, grandCost float64, totalW int) {
	fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(ColorSky).Render("📁 Project / Workspace Cost Attribution (--by-project)"))

	cols := []struct {
		name  string
		width int
		align lipgloss.Position
	}{
		{"Workspace / Repository", 51, lipgloss.Left},
		{"Requests", 10, lipgloss.Right},
		{"Tokens", 12, lipgloss.Right},
		{"Cost ($)", 12, lipgloss.Right},
		{"Share", 16, lipgloss.Right},
	}

	var b strings.Builder
	b.WriteString(renderTableBorder("┌", "┬", "┐", cols))
	b.WriteString("\n")

	var hCells []string
	for _, c := range cols {
		hCells = append(hCells, lipgloss.NewStyle().Width(c.width).Align(c.align).Render(StyleTableHeader.Render(c.name)))
	}
	b.WriteString(renderTableRow(hCells))
	b.WriteString("\n")
	b.WriteString(renderTableBorder("├", "┼", "┤", cols))
	b.WriteString("\n")

	for _, p := range projects {
		bar := MakeProgressBar(p.SharePct, 6)
		cells := []string{
			lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(lipgloss.NewStyle().Foreground(ColorText).Render(truncate(p.DisplayName, cols[0].width))),
			lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(formatInt(p.Requests)),
			lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(StyleSecondary.Render(FormatTokens(p.InputTokens + p.OutputTokens + p.CacheTokens))),
			lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(StyleCost.Render(FormatCost(p.Cost))),
			lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(fmt.Sprintf("%s %4.1f%%", bar, p.SharePct)),
		}
		b.WriteString(renderTableRow(cells))
		b.WriteString("\n")
	}
	b.WriteString(renderTableBorder("└", "┴", "┘", cols))
	fmt.Println(b.String())
	fmt.Println()
}

// RenderAgentAttributionTable renders cost broken down by main vs subagent vs advisor.
func RenderAgentAttributionTable(agents []*model.AgentTypeCost, grandCost float64, totalW int) {
	fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(ColorMauve).Render("🤖 Agent Architecture Attribution (--by-agent)"))

	cols := []struct {
		name  string
		width int
		align lipgloss.Position
	}{
		{"Agent Role", 49, lipgloss.Left},
		{"Requests", 10, lipgloss.Right},
		{"Tokens", 14, lipgloss.Right},
		{"Cost ($)", 12, lipgloss.Right},
		{"Share", 16, lipgloss.Right},
	}

	var b strings.Builder
	b.WriteString(renderTableBorder("┌", "┬", "┐", cols))
	b.WriteString("\n")

	var hCells []string
	for _, c := range cols {
		hCells = append(hCells, lipgloss.NewStyle().Width(c.width).Align(c.align).Render(StyleTableHeader.Render(c.name)))
	}
	b.WriteString(renderTableRow(hCells))
	b.WriteString("\n")
	b.WriteString(renderTableBorder("├", "┼", "┤", cols))
	b.WriteString("\n")

	for _, a := range agents {
		displayName := a.AgentType
		switch a.AgentType {
		case "main":
			displayName = "Main Orchestrator"
		case "subagent":
			displayName = "Parallel Subagent"
		case "advisor":
			displayName = "Background Advisor"
		}

		bar := MakeProgressBar(a.SharePct, 6)
		cells := []string{
			lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(lipgloss.NewStyle().Bold(true).Foreground(ColorMauve).Render(displayName)),
			lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(formatInt(a.Requests)),
			lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(StyleSecondary.Render(FormatTokens(a.InputTokens + a.OutputTokens + a.CacheTokens))),
			lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(StyleCost.Render(FormatCost(a.Cost))),
			lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(fmt.Sprintf("%s %4.1f%%", bar, a.SharePct)),
		}
		b.WriteString(renderTableRow(cells))
		b.WriteString("\n")
	}
	b.WriteString(renderTableBorder("└", "┴", "┘", cols))
	fmt.Println(b.String())
	fmt.Println()
}

// RenderPerformanceBenchmarkTable renders latency & throughput performance metrics per model.
func RenderPerformanceBenchmarkTable(models []*model.ModelCost, totalW int) {
	fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(ColorYellow).Render("⚡ Latency & Throughput Benchmark (--perf)"))

	cols := []struct {
		name  string
		width int
		align lipgloss.Position
	}{
		{"Provider / Model", 53, lipgloss.Left},
		{"Requests", 9, lipgloss.Right},
		{"Avg TTFT", 12, lipgloss.Right},
		{"Avg Latency", 13, lipgloss.Right},
		{"Throughput", 14, lipgloss.Right},
	}

	var b strings.Builder
	b.WriteString(renderTableBorder("┌", "┬", "┐", cols))
	b.WriteString("\n")

	var hCells []string
	for _, c := range cols {
		hCells = append(hCells, lipgloss.NewStyle().Width(c.width).Align(c.align).Render(StyleTableHeader.Render(c.name)))
	}
	b.WriteString(renderTableRow(hCells))
	b.WriteString("\n")
	b.WriteString(renderTableBorder("├", "┼", "┤", cols))
	b.WriteString("\n")

	for _, m := range models {
		if m.Requests == 0 {
			continue
		}
		ttftStr := fmt.Sprintf("%.0f ms", m.AvgTtftMs)
		latStr := fmt.Sprintf("%.2f s", m.AvgDurationMs/1000.0)
		tpsStr := fmt.Sprintf("%.1f tok/s", m.TokensPerSec())

		cells := []string{
			lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(fmt.Sprintf("%s / %s", StyleSecondary.Render(m.Provider), StyleModel.Render(m.Model))),
			lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(formatInt(m.Requests)),
			lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(lipgloss.NewStyle().Foreground(ColorSky).Render(ttftStr)),
			lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(StyleSecondary.Render(latStr)),
			lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(lipgloss.NewStyle().Bold(true).Foreground(ColorGreen).Render(tpsStr)),
		}
		b.WriteString(renderTableRow(cells))
		b.WriteString("\n")
	}
	b.WriteString(renderTableBorder("└", "┴", "┘", cols))
	fmt.Println(b.String())
	fmt.Println()
}

// RenderDailyBreakdownTable renders day-by-day cost and token breakdown.
func RenderDailyBreakdownTable(days []*model.DailyCost, grandCost float64, totalW int) {
	fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(ColorSky).Render("📅 Daily Cost & Usage Breakdown (--daily)"))

	cols := []struct {
		name  string
		width int
		align lipgloss.Position
	}{
		{"Date", 14, lipgloss.Left},
		{"Day", 8, lipgloss.Left},
		{"Requests", 11, lipgloss.Right},
		{"In / Out Tokens", 18, lipgloss.Right},
		{"Cache Hit", 13, lipgloss.Right},
		{"Cost ($)", 14, lipgloss.Right},
		{"Share", 17, lipgloss.Right},
	}

	var b strings.Builder
	b.WriteString(renderTableBorder("┌", "┬", "┐", cols))
	b.WriteString("\n")

	var hCells []string
	for _, c := range cols {
		hCells = append(hCells, lipgloss.NewStyle().Width(c.width).Align(c.align).Render(StyleTableHeader.Render(c.name)))
	}
	b.WriteString(renderTableRow(hCells))
	b.WriteString("\n")
	b.WriteString(renderTableBorder("├", "┼", "┤", cols))
	b.WriteString("\n")

	for _, d := range days {
		bar := MakeProgressBar(d.SharePct, 6)
		cHit := fmt.Sprintf("%.1f%%", d.CacheHitRate())
		cStyle := StyleSecondary
		if d.CacheHitRate() >= 90.0 {
			cStyle = lipgloss.NewStyle().Foreground(ColorGreen)
		} else if d.CacheHitRate() > 0 {
			cStyle = lipgloss.NewStyle().Foreground(ColorYellow)
		}

		cells := []string{
			lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(lipgloss.NewStyle().Bold(true).Foreground(ColorSky).Render(d.Date)),
			lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(StyleSecondary.Render(d.DayOfWeek)),
			lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(formatInt(d.Requests)),
			lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(StyleSecondary.Render(fmt.Sprintf("%s/%s", FormatTokens(d.InputTokens), FormatTokens(d.OutputTokens)))),
			lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(cStyle.Render(cHit)),
			lipgloss.NewStyle().Width(cols[5].width).Align(cols[5].align).Render(StyleCost.Render(FormatCost(d.Cost))),
			lipgloss.NewStyle().Width(cols[6].width).Align(cols[6].align).Render(fmt.Sprintf("%s %4.1f%%", bar, d.SharePct)),
		}
		b.WriteString(renderTableRow(cells))
		b.WriteString("\n")
	}
	b.WriteString(renderTableBorder("└", "┴", "┘", cols))
	fmt.Println(b.String())
	fmt.Println()
}

// RenderWeeklyBreakdownTable renders week-by-week cost and token breakdown.
func RenderWeeklyBreakdownTable(weeks []*model.WeeklyCost, grandCost float64, totalW int) {
	fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Render("📆 Weekly Cost & Usage Breakdown (--weekly)"))

	cols := []struct {
		name  string
		width int
		align lipgloss.Position
	}{
		{"Week", 14, lipgloss.Left},
		{"Date Range", 18, lipgloss.Left},
		{"Requests", 10, lipgloss.Right},
		{"In / Out Tokens", 16, lipgloss.Right},
		{"Cache Hit", 11, lipgloss.Right},
		{"Cost ($)", 13, lipgloss.Right},
		{"Share", 13, lipgloss.Right},
	}

	var b strings.Builder
	b.WriteString(renderTableBorder("┌", "┬", "┐", cols))
	b.WriteString("\n")

	var hCells []string
	for _, c := range cols {
		hCells = append(hCells, lipgloss.NewStyle().Width(c.width).Align(c.align).Render(StyleTableHeader.Render(c.name)))
	}
	b.WriteString(renderTableRow(hCells))
	b.WriteString("\n")
	b.WriteString(renderTableBorder("├", "┼", "┤", cols))
	b.WriteString("\n")

	for _, wItem := range weeks {
		bar := MakeProgressBar(wItem.SharePct, 6)
		cHit := fmt.Sprintf("%.1f%%", wItem.CacheHitRate())
		cStyle := StyleSecondary
		if wItem.CacheHitRate() >= 90.0 {
			cStyle = lipgloss.NewStyle().Foreground(ColorGreen)
		} else if wItem.CacheHitRate() > 0 {
			cStyle = lipgloss.NewStyle().Foreground(ColorYellow)
		}

		cells := []string{
			lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Render(wItem.Week)),
			lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(StyleSecondary.Render(wItem.DateRange)),
			lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(formatInt(wItem.Requests)),
			lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(StyleSecondary.Render(fmt.Sprintf("%s/%s", FormatTokens(wItem.InputTokens), FormatTokens(wItem.OutputTokens)))),
			lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(cStyle.Render(cHit)),
			lipgloss.NewStyle().Width(cols[5].width).Align(cols[5].align).Render(StyleCost.Render(FormatCost(wItem.Cost))),
			lipgloss.NewStyle().Width(cols[6].width).Align(cols[6].align).Render(fmt.Sprintf("%s %4.1f%%", bar, wItem.SharePct)),
		}
		b.WriteString(renderTableRow(cells))
		b.WriteString("\n")
	}
	b.WriteString(renderTableBorder("└", "┴", "┘", cols))
	fmt.Println(b.String())
	fmt.Println()
}

func renderTableRow(cells []string) string {
	sep := lipgloss.NewStyle().Foreground(ColorLine).Render("│")
	return fmt.Sprintf("%s %s %s", sep, strings.Join(cells, fmt.Sprintf(" %s ", sep)), sep)
}

func renderTableBorder(left, mid, right string, cols []struct {
	name  string
	width int
	align lipgloss.Position
}) string {
	var parts []string
	for _, c := range cols {
		parts = append(parts, strings.Repeat("─", c.width+2))
	}
	borderStyle := lipgloss.NewStyle().Foreground(ColorLine)
	return borderStyle.Render(left + strings.Join(parts, mid) + right)
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}

// Helper method on summary
func countModels(s *model.ProfileSummary) int {
	count := 0
	for _, p := range s.Providers {
		count += len(p.Models)
	}
	return count
}
