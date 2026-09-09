package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faridlamaul/omp-cost/internal/db"
	"github.com/faridlamaul/omp-cost/internal/model"
)

type tabIndex int

const (
	tabModels tabIndex = iota
	tabDaily
	tabWeekly
	tabProjects
	tabPerf
)

var tabNames = []string{
	"1: Models",
	"2: Daily",
	"3: Weekly",
	"4: Projects",
	"5: Performance",
}

// InteractiveModel holds the state for the Bubbletea interactive TUI.
type InteractiveModel struct {
	reader             *db.Reader
	allProfiles        map[string]string
	profileNames       []string
	selectedProfileIdx int
	months             []string
	selectedMonthIdx   int
	activeTab          tabIndex
	width              int
	height             int
	scrollOffset       int
	report             *model.MultiProfileReport
}

// NewInteractiveModel initializes the interactive state.
func NewInteractiveModel(reader *db.Reader, allProfiles map[string]string, initialProfile, initialMonth string) *InteractiveModel {
	var pNames []string
	pNames = append(pNames, "all")
	for p := range allProfiles {
		pNames = append(pNames, p)
	}
	sort.Strings(pNames[1:]) // Keep "all" first

	// Discover all historical months across databases
	monthSet := make(map[string]bool)
	for _, dbPath := range allProfiles {
		ms, err := reader.GetDistinctMonths(dbPath)
		if err == nil {
			for _, m := range ms {
				monthSet[m] = true
			}
		}
	}
	var months []string
	for m := range monthSet {
		months = append(months, m)
	}
	sort.Slice(months, func(i, j int) bool {
		return months[i] > months[j]
	})
	if len(months) == 0 {
		months = []string{"2026-09"}
	}

	pIdx := 0
	for i, name := range pNames {
		if name == initialProfile {
			pIdx = i
			break
		}
	}

	mIdx := 0
	for i, m := range months {
		if m == initialMonth {
			mIdx = i
			break
		}
	}

	m := &InteractiveModel{
		reader:             reader,
		allProfiles:        allProfiles,
		profileNames:       pNames,
		selectedProfileIdx: pIdx,
		months:             months,
		selectedMonthIdx:   mIdx,
		activeTab:          tabModels,
		width:              108,
		height:             32,
	}

	m.loadData()
	return m
}

func (m *InteractiveModel) loadData() {
	currentMonth := m.months[m.selectedMonthIdx]
	currentProfile := m.profileNames[m.selectedProfileIdx]

	report := &model.MultiProfileReport{
		Month: currentMonth,
	}

	if currentProfile == "all" {
		for _, name := range m.profileNames[1:] {
			dbPath := m.allProfiles[name]
			summary, err := m.reader.BuildProfileSummary(name, dbPath, currentMonth, 0)
			if err == nil && summary != nil {
				report.Profiles = append(report.Profiles, summary)
				report.TotalRequests += summary.Requests
				report.TotalInput += summary.InputTokens
				report.TotalOutput += summary.OutputTokens
				report.TotalCache += summary.CacheTokens()
				report.TotalCost += summary.Cost
				report.TotalCostNoCache += summary.CostNoCache
			}
		}
		sort.Slice(report.Profiles, func(i, j int) bool {
			return report.Profiles[i].Cost > report.Profiles[j].Cost
		})
	} else {
		dbPath := m.allProfiles[currentProfile]
		summary, err := m.reader.BuildProfileSummary(currentProfile, dbPath, currentMonth, 0)
		if err == nil && summary != nil {
			report.Profiles = append(report.Profiles, summary)
			report.TotalRequests = summary.Requests
			report.TotalInput = summary.InputTokens
			report.TotalOutput = summary.OutputTokens
			report.TotalCache = summary.CacheTokens()
			report.TotalCost = summary.Cost
			report.TotalCostNoCache = summary.CostNoCache
		}
	}

	savedDollars := report.TotalCostNoCache - report.TotalCost
	if savedDollars < 0 {
		savedDollars = 0
	}
	savingsPct := 0.0
	if report.TotalCostNoCache > 0 {
		savingsPct = (savedDollars / report.TotalCostNoCache) * 100.0
	}
	report.TotalSavings = model.CacheSavings{
		NetSpend:         report.TotalCost,
		EstimatedNoCache: report.TotalCostNoCache,
		DollarsSaved:     savedDollars,
		SavingsPct:       savingsPct,
	}

	m.report = report
	m.scrollOffset = 0
}

func (m *InteractiveModel) Init() tea.Cmd {
	return nil
}

func (m *InteractiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit

		case "left", "h":
			if m.selectedMonthIdx < len(m.months)-1 {
				m.selectedMonthIdx++
				m.loadData()
			}

		case "right", "l":
			if m.selectedMonthIdx > 0 {
				m.selectedMonthIdx--
				m.loadData()
			}

		case "[":
			if m.selectedProfileIdx > 0 {
				m.selectedProfileIdx--
			} else {
				m.selectedProfileIdx = len(m.profileNames) - 1
			}
			m.loadData()

		case "]":
			m.selectedProfileIdx = (m.selectedProfileIdx + 1) % len(m.profileNames)
			m.loadData()

		case "tab":
			m.activeTab = (m.activeTab + 1) % 5
			m.scrollOffset = 0

		case "shift+tab":
			if m.activeTab > 0 {
				m.activeTab--
			} else {
				m.activeTab = 4
			}
			m.scrollOffset = 0

		case "1":
			m.activeTab = tabModels
			m.scrollOffset = 0
		case "2":
			m.activeTab = tabDaily
			m.scrollOffset = 0
		case "3":
			m.activeTab = tabWeekly
			m.scrollOffset = 0
		case "4":
			m.activeTab = tabProjects
			m.scrollOffset = 0
		case "5":
			m.activeTab = tabPerf
			m.scrollOffset = 0

		case "up", "k":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}

		case "down", "j":
			m.scrollOffset++

		case "r":
			m.loadData()
		}
	}

	return m, nil
}

func (m *InteractiveModel) View() string {
	var b strings.Builder
	w := m.width
	if w < 80 {
		w = 80
	}
	if w > 118 {
		w = 118
	}

	// 1. Header & Navigation Controls
	currentMonth := m.months[m.selectedMonthIdx]
	currentProfile := m.profileNames[m.selectedProfileIdx]

	monthNav := fmt.Sprintf("◀  %s  ▶", currentMonth)
	if m.selectedMonthIdx == len(m.months)-1 {
		monthNav = fmt.Sprintf("   %s  ▶", currentMonth)
	} else if m.selectedMonthIdx == 0 {
		monthNav = fmt.Sprintf("◀  %s   ", currentMonth)
	}

	header := fmt.Sprintf("%s  •  %s %s  •  %s %s",
		StyleHeaderTitle.Render("⚡ OH MY PI INTERACTIVE"),
		StyleSecondary.Render("Profile:"),
		lipgloss.NewStyle().Bold(true).Foreground(ColorSky).Render(fmt.Sprintf("[%s]", currentProfile)),
		StyleSecondary.Render("Month:"),
		lipgloss.NewStyle().Bold(true).Foreground(ColorYellow).Render(monthNav),
	)
	b.WriteString(StyleCard.Width(w - 2).Render(header))
	b.WriteString("\n\n")

	// 2. Tab Bar
	var tabItems []string
	for i, name := range tabNames {
		style := lipgloss.NewStyle().Padding(0, 1)
		if tabIndex(i) == m.activeTab {
			style = style.Bold(true).Foreground(ColorText).Background(ColorSurface)
		} else {
			style = style.Foreground(ColorSubtext)
		}
		tabItems = append(tabItems, style.Render(name))
	}
	tabBar := strings.Join(tabItems, " ")
	b.WriteString(tabBar)
	b.WriteString("\n\n")

	// 3. Quick KPI Row
	costStr := FormatCost(m.report.TotalCost)
	reqsStr := fmt.Sprintf("%s reqs", formatInt(m.report.TotalRequests))
	cacheStr := fmt.Sprintf("%.1f%% (%s)", m.report.OverallCacheHitRate(), FormatTokens(m.report.TotalCache))
	savedStr := fmt.Sprintf("Saved: %s", FormatCost(m.report.TotalSavings.DollarsSaved))

	kpiLine := fmt.Sprintf("%s %s   %s %s   %s %s   %s %s",
		StyleSecondary.Render("Cost:"), StyleCost.Render(costStr),
		StyleSecondary.Render("Requests:"), StyleBold.Render(reqsStr),
		StyleSecondary.Render("Cache Hit:"), lipgloss.NewStyle().Foreground(ColorSky).Render(cacheStr),
		StyleSecondary.Render("Savings:"), lipgloss.NewStyle().Bold(true).Foreground(ColorGreen).Render(savedStr),
	)
	b.WriteString(kpiLine)
	b.WriteString("\n\n")

	// 4. View based on activeTab
	var content strings.Builder
	switch m.activeTab {
	case tabModels:
		if currentProfile == "all" {
			content.WriteString(renderProfilesSummaryTable(m.report.Profiles, m.report.TotalCost, w))
			content.WriteString("\n")
			for _, p := range m.report.Profiles {
				if p.Requests > 0 {
					content.WriteString(fmt.Sprintf("\n%s\n", lipgloss.NewStyle().Bold(true).Foreground(ColorMauve).Render(fmt.Sprintf("▶ Profile: %s (%s)", p.Profile, FormatCost(p.Cost)))))
					content.WriteString(renderModelBreakdownTable(p, w))
				}
			}
		} else if len(m.report.Profiles) > 0 {
			content.WriteString(renderModelBreakdownTable(m.report.Profiles[0], w))
		}

	case tabDaily:
		for _, p := range m.report.Profiles {
			if len(p.Days) > 0 {
				content.WriteString(fmt.Sprintf("%s (%s)\n", lipgloss.NewStyle().Bold(true).Foreground(ColorSky).Render(fmt.Sprintf("📅 Daily: %s", p.Profile)), FormatCost(p.Cost)))
				var dayLines strings.Builder
				cols := []struct {
					name  string
					width int
					align lipgloss.Position
				}{
					{"Date", 12, lipgloss.Left},
					{"Day", 6, lipgloss.Left},
					{"Requests", 9, lipgloss.Right},
					{"In / Out Tokens", 16, lipgloss.Right},
					{"Cache Hit", 11, lipgloss.Right},
					{"Cost ($)", 12, lipgloss.Right},
					{"Share", 16, lipgloss.Right},
				}
				dayLines.WriteString(renderTableBorder("┌", "┬", "┐", cols))
				dayLines.WriteString("\n")
				var hCells []string
				for _, c := range cols {
					hCells = append(hCells, lipgloss.NewStyle().Width(c.width).Align(c.align).Render(StyleTableHeader.Render(c.name)))
				}
				dayLines.WriteString(renderTableRow(hCells))
				dayLines.WriteString("\n")
				dayLines.WriteString(renderTableBorder("├", "┼", "┤", cols))
				dayLines.WriteString("\n")

				for _, d := range p.Days {
					bar := MakeProgressBar(d.SharePct, 6)
					cHit := fmt.Sprintf("%.1f%%", d.CacheHitRate())
					cStyle := StyleSecondary
					if d.CacheHitRate() >= 90.0 {
						cStyle = lipgloss.NewStyle().Foreground(ColorGreen)
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
					dayLines.WriteString(renderTableRow(cells))
					dayLines.WriteString("\n")
				}
				dayLines.WriteString(renderTableBorder("└", "┴", "┘", cols))
				content.WriteString(dayLines.String())
				content.WriteString("\n\n")
			}
		}

	case tabWeekly:
		for _, p := range m.report.Profiles {
			if len(p.Weeks) > 0 {
				content.WriteString(fmt.Sprintf("%s (%s)\n", lipgloss.NewStyle().Bold(true).Foreground(ColorPeach).Render(fmt.Sprintf("📆 Weekly: %s", p.Profile)), FormatCost(p.Cost)))
				var wkLines strings.Builder
				cols := []struct {
					name  string
					width int
					align lipgloss.Position
				}{
					{"Week", 12, lipgloss.Left},
					{"Date Range", 18, lipgloss.Left},
					{"Requests", 9, lipgloss.Right},
					{"In / Out Tokens", 16, lipgloss.Right},
					{"Cache Hit", 11, lipgloss.Right},
					{"Cost ($)", 12, lipgloss.Right},
					{"Share", 16, lipgloss.Right},
				}
				wkLines.WriteString(renderTableBorder("┌", "┬", "┐", cols))
				wkLines.WriteString("\n")
				var hCells []string
				for _, c := range cols {
					hCells = append(hCells, lipgloss.NewStyle().Width(c.width).Align(c.align).Render(StyleTableHeader.Render(c.name)))
				}
				wkLines.WriteString(renderTableRow(hCells))
				wkLines.WriteString("\n")
				wkLines.WriteString(renderTableBorder("├", "┼", "┤", cols))
				wkLines.WriteString("\n")

				for _, wItem := range p.Weeks {
					bar := MakeProgressBar(wItem.SharePct, 6)
					cHit := fmt.Sprintf("%.1f%%", wItem.CacheHitRate())
					cStyle := StyleSecondary
					if wItem.CacheHitRate() >= 90.0 {
						cStyle = lipgloss.NewStyle().Foreground(ColorGreen)
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
					wkLines.WriteString(renderTableRow(cells))
					wkLines.WriteString("\n")
				}
				wkLines.WriteString(renderTableBorder("└", "┴", "┘", cols))
				content.WriteString(wkLines.String())
				content.WriteString("\n\n")
			}
		}

	case tabProjects:
		for _, p := range m.report.Profiles {
			if len(p.Projects) > 0 {
				content.WriteString(fmt.Sprintf("%s\n", lipgloss.NewStyle().Bold(true).Foreground(ColorSky).Render(fmt.Sprintf("📁 Projects: %s", p.Profile))))
				var prjLines strings.Builder
				cols := []struct {
					name  string
					width int
					align lipgloss.Position
				}{
					{"Workspace / Repository", 36, lipgloss.Left},
					{"Requests", 10, lipgloss.Right},
					{"Tokens", 12, lipgloss.Right},
					{"Cost ($)", 12, lipgloss.Right},
					{"Share", 16, lipgloss.Right},
				}
				prjLines.WriteString(renderTableBorder("┌", "┬", "┐", cols))
				prjLines.WriteString("\n")
				var hCells []string
				for _, c := range cols {
					hCells = append(hCells, lipgloss.NewStyle().Width(c.width).Align(c.align).Render(StyleTableHeader.Render(c.name)))
				}
				prjLines.WriteString(renderTableRow(hCells))
				prjLines.WriteString("\n")
				prjLines.WriteString(renderTableBorder("├", "┼", "┤", cols))
				prjLines.WriteString("\n")

				for _, prj := range p.Projects {
					bar := MakeProgressBar(prj.SharePct, 6)
					cells := []string{
						lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(lipgloss.NewStyle().Foreground(ColorText).Render(truncate(prj.DisplayName, cols[0].width))),
						lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(formatInt(prj.Requests)),
						lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(StyleSecondary.Render(FormatTokens(prj.InputTokens + prj.OutputTokens + prj.CacheTokens))),
						lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(StyleCost.Render(FormatCost(prj.Cost))),
						lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(fmt.Sprintf("%s %4.1f%%", bar, prj.SharePct)),
					}
					prjLines.WriteString(renderTableRow(cells))
					prjLines.WriteString("\n")
				}
				prjLines.WriteString(renderTableBorder("└", "┴", "┘", cols))
				content.WriteString(prjLines.String())
				content.WriteString("\n\n")
			}
		}

	case tabPerf:
		for _, p := range m.report.Profiles {
			var models []*model.ModelCost
			for _, prov := range p.Providers {
				models = append(models, prov.Models...)
			}
			if len(models) > 0 {
				content.WriteString(fmt.Sprintf("%s\n", lipgloss.NewStyle().Bold(true).Foreground(ColorYellow).Render(fmt.Sprintf("⚡ Performance: %s", p.Profile))))
				var perfLines strings.Builder
				cols := []struct {
					name  string
					width int
					align lipgloss.Position
				}{
					{"Provider / Model", 38, lipgloss.Left},
					{"Requests", 9, lipgloss.Right},
					{"Avg TTFT", 12, lipgloss.Right},
					{"Avg Latency", 13, lipgloss.Right},
					{"Throughput", 14, lipgloss.Right},
				}
				perfLines.WriteString(renderTableBorder("┌", "┬", "┐", cols))
				perfLines.WriteString("\n")
				var hCells []string
				for _, c := range cols {
					hCells = append(hCells, lipgloss.NewStyle().Width(c.width).Align(c.align).Render(StyleTableHeader.Render(c.name)))
				}
				perfLines.WriteString(renderTableRow(hCells))
				perfLines.WriteString("\n")
				perfLines.WriteString(renderTableBorder("├", "┼", "┤", cols))
				perfLines.WriteString("\n")

				for _, mc := range models {
					if mc.Requests == 0 {
						continue
					}
					cells := []string{
						lipgloss.NewStyle().Width(cols[0].width).Align(cols[0].align).Render(fmt.Sprintf("%s / %s", StyleSecondary.Render(mc.Provider), StyleModel.Render(mc.Model))),
						lipgloss.NewStyle().Width(cols[1].width).Align(cols[1].align).Render(formatInt(mc.Requests)),
						lipgloss.NewStyle().Width(cols[2].width).Align(cols[2].align).Render(lipgloss.NewStyle().Foreground(ColorSky).Render(fmt.Sprintf("%.0f ms", mc.AvgTtftMs))),
						lipgloss.NewStyle().Width(cols[3].width).Align(cols[3].align).Render(StyleSecondary.Render(fmt.Sprintf("%.2f s", mc.AvgDurationMs/1000.0))),
						lipgloss.NewStyle().Width(cols[4].width).Align(cols[4].align).Render(lipgloss.NewStyle().Bold(true).Foreground(ColorGreen).Render(fmt.Sprintf("%.1f tok/s", mc.TokensPerSec()))),
					}
					perfLines.WriteString(renderTableRow(cells))
					perfLines.WriteString("\n")
				}
				perfLines.WriteString(renderTableBorder("└", "┴", "┘", cols))
				content.WriteString(perfLines.String())
				content.WriteString("\n\n")
			}
		}
	}

	// Scroll management
	lines := strings.Split(content.String(), "\n")
	maxVisible := m.height - 10
	if maxVisible < 8 {
		maxVisible = 8
	}

	start := m.scrollOffset
	if start > len(lines)-maxVisible {
		start = len(lines) - maxVisible
	}
	if start < 0 {
		start = 0
	}

	end := start + maxVisible
	if end > len(lines) {
		end = len(lines)
	}

	b.WriteString(strings.Join(lines[start:end], "\n"))
	b.WriteString("\n\n")

	// 5. Footer Hotkey Guide
	footer := fmt.Sprintf("%s %s %s %s %s %s %s",
		StyleDim.Render("Hotkeys:"),
		lipgloss.NewStyle().Foreground(ColorSky).Render("◀/▶ Month"),
		lipgloss.NewStyle().Foreground(ColorMauve).Render("[/] Profile"),
		lipgloss.NewStyle().Foreground(ColorYellow).Render("Tab/1-5 View"),
		lipgloss.NewStyle().Foreground(ColorText).Render("↑/↓ Scroll"),
		lipgloss.NewStyle().Foreground(ColorGreen).Render("r Reload"),
		lipgloss.NewStyle().Foreground(ColorRed).Render("q Quit"),
	)
	b.WriteString(footer)

	return b.String()
}

// RunInteractive launches the Bubbletea interactive program.
func RunInteractive(reader *db.Reader, allProfiles map[string]string, initialProfile, initialMonth string) error {
	m := NewInteractiveModel(reader, allProfiles, initialProfile, initialMonth)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
