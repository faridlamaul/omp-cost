package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/faridlamaul/omp-cost/internal/db"
	"github.com/faridlamaul/omp-cost/internal/export"
	"github.com/faridlamaul/omp-cost/internal/model"
	"github.com/faridlamaul/omp-cost/internal/ui"
	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"

	flagOutput      string
	flagJSON        bool
	flagProfile     string
	flagMonth       string
	flagAllProfiles bool
	flagByProject   bool
	flagByAgent     bool
	flagPerf        bool
	flagSavings     bool
	flagForecast    bool
	flagBudget      float64
	flagSync        bool
	flagNoSync      bool
	flagList        bool
	flagVersion     bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "omp-cost [PROFILE] [MONTH] [FLAGS]",
		Short: "Analyze AI token usage and costs across Oh My Pi profiles by calendar month",
		Long: `⚡ OH MY PI — Usage Cost Analyzer (omp-cost)

Inspect detailed token usage, prompt caching efficiency, and API costs
broken down by calendar month for individual profiles or all profiles.`,
		Example: `  # Summary of all profiles for the current month:
  omp-cost

  # Detailed breakdown for a specific profile in current month:
  omp-cost hermes
  omp-cost germatech

  # Detailed breakdown for a specific profile in a specific month:
  omp-cost hermes 2026-08
  omp-cost germatech 2026-09

  # Breakdown by workspace repository or project:
  omp-cost --by-project

  # Breakdown by agent architecture (orchestrator vs subagents):
  omp-cost --by-agent

  # Latency & Throughput benchmark:
  omp-cost --perf

  # Prompt cache dollar savings:
  omp-cost --savings

  # Daily burn-rate and month-end forecast with budget alert:
  omp-cost --forecast --budget 300

  # Export data to different formats:
  omp-cost -o json
  omp-cost -o csv > costs.csv
  omp-cost -o md`,
		RunE: runRoot,
	}

	rootCmd.Flags().StringVarP(&flagOutput, "output", "o", "text", "Output format: text, json, csv, md")
	rootCmd.Flags().BoolVar(&flagJSON, "json", false, "Shortcut for '--output json'")
	rootCmd.Flags().StringVarP(&flagProfile, "profile", "p", "", "Target profile name")
	rootCmd.Flags().StringVarP(&flagMonth, "month", "m", "", "Target calendar month (YYYY-MM or all)")
	rootCmd.Flags().BoolVarP(&flagAllProfiles, "all-profiles", "a", false, "Display aggregated overview across all profiles")
	rootCmd.Flags().BoolVar(&flagByProject, "by-project", false, "Show cost attribution by workspace/repository")
	rootCmd.Flags().BoolVar(&flagByAgent, "by-agent", false, "Show cost attribution by agent architecture (main, subagent, advisor)")
	rootCmd.Flags().BoolVar(&flagPerf, "perf", false, "Show latency and throughput performance benchmarks")
	rootCmd.Flags().BoolVar(&flagSavings, "savings", false, "Show prompt caching ROI and dollar savings")
	rootCmd.Flags().BoolVar(&flagForecast, "forecast", false, "Show daily burn-rate and month-end cost projection")
	rootCmd.Flags().Float64Var(&flagBudget, "budget", 0.0, "Set monthly budget cap for forecast alert")
	rootCmd.Flags().BoolVar(&flagSync, "sync", false, "Force sync sessions from omp transcripts before querying")
	rootCmd.Flags().BoolVar(&flagNoSync, "no-sync", false, "Skip background session sync for ultra-fast query (<15ms)")
	rootCmd.Flags().BoolVarP(&flagList, "list", "l", false, "List all discovered profiles and database paths")
	rootCmd.Flags().BoolVarP(&flagVersion, "version", "v", false, "Print version information")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	if flagVersion {
		fmt.Printf("omp-cost v%s\n", version)
		return nil
	}

	reader := db.NewReader()
	allProfiles := reader.DiscoverProfiles()

	if flagList {
		fmt.Println("\nDetected Oh My Pi Profiles:")
		var names []string
		for name := range allProfiles {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("  • %-14s -> %s\n", name, allProfiles[name])
		}
		fmt.Println()
		return nil
	}

	monthRegex := regexp.MustCompile(`^\d{4}-\d{2}$`)

	// Resolve positional arguments flexibly
	targetProfile := flagProfile
	targetMonth := flagMonth

	for _, arg := range args {
		if monthRegex.MatchString(arg) {
			if targetMonth == "" {
				targetMonth = arg
			}
		} else if arg == "all" {
			if targetProfile == "" && !flagAllProfiles {
				targetProfile = "all"
			} else if targetMonth == "" {
				targetMonth = "all"
			}
		} else {
			if targetProfile == "" {
				targetProfile = arg
			} else if targetMonth == "" {
				targetMonth = arg
			}
		}
	}

	if flagAllProfiles {
		targetProfile = "all"
	}

	if targetProfile == "" {
		targetProfile = "all"
	}
	if targetMonth == "" {
		targetMonth = time.Now().Format("2006-01")
	}

	// Concurrent session sync unless --no-sync is specified
	if !flagNoSync {
		var profilesToSync []string
		if targetProfile == "all" {
			for name := range allProfiles {
				profilesToSync = append(profilesToSync, name)
			}
		} else {
			profilesToSync = append(profilesToSync, targetProfile)
		}
		reader.SyncProfilesConcurrently(profilesToSync)
	}

	// Format resolution
	outputFmt := strings.ToLower(flagOutput)
	if flagJSON {
		outputFmt = "json"
	}

	renderFlags := ui.RenderFlags{
		ShowProjects: flagByProject,
		ShowAgents:   flagByAgent,
		ShowPerf:     flagPerf,
		ShowSavings:  flagSavings,
		ShowForecast: flagForecast,
		BudgetCap:    flagBudget,
	}

	if targetProfile == "all" {
		return handleAllProfiles(reader, allProfiles, targetMonth, outputFmt, renderFlags)
	}

	return handleSingleProfile(reader, allProfiles, targetProfile, targetMonth, outputFmt, renderFlags)
}

func handleAllProfiles(reader *db.Reader, allProfiles map[string]string, month string, outputFmt string, flags ui.RenderFlags) error {
	var profileNames []string
	for name := range allProfiles {
		profileNames = append(profileNames, name)
	}
	sort.Strings(profileNames)

	if month == "all" {
		// Collect all historical months across all profiles
		monthSet := make(map[string]bool)
		for _, dbPath := range allProfiles {
			ms, err := reader.GetDistinctMonths(dbPath)
			if err == nil {
				for _, m := range ms {
					monthSet[m] = true
				}
			}
		}

		var sortedMonths []string
		for m := range monthSet {
			sortedMonths = append(sortedMonths, m)
		}
		sort.Slice(sortedMonths, func(i, j int) bool {
			return sortedMonths[i] > sortedMonths[j]
		})

		if len(sortedMonths) == 0 {
			fmt.Println("\nℹ No usage data recorded in database.")
			return nil
		}

		for _, m := range sortedMonths {
			report := buildMultiReport(reader, allProfiles, profileNames, m, flags.BudgetCap)
			if err := outputReport(report, outputFmt, flags); err != nil {
				return err
			}
		}
		return nil
	}

	report := buildMultiReport(reader, allProfiles, profileNames, month, flags.BudgetCap)
	return outputReport(report, outputFmt, flags)
}

func buildMultiReport(reader *db.Reader, allProfiles map[string]string, profileNames []string, month string, budgetCap float64) *model.MultiProfileReport {
	report := &model.MultiProfileReport{
		Month: month,
	}

	for _, name := range profileNames {
		dbPath := allProfiles[name]
		summary, err := reader.BuildProfileSummary(name, dbPath, month, budgetCap)
		if err != nil || summary == nil {
			summary = &model.ProfileSummary{
				Profile: name,
				Month:   month,
			}
		}
		report.Profiles = append(report.Profiles, summary)
		report.TotalRequests += summary.Requests
		report.TotalInput += summary.InputTokens
		report.TotalOutput += summary.OutputTokens
		report.TotalCache += summary.CacheTokens()
		report.TotalCost += summary.Cost
		report.TotalCostNoCache += summary.CostNoCache
	}

	// Sort profiles by spend descending
	sort.Slice(report.Profiles, func(i, j int) bool {
		return report.Profiles[i].Cost > report.Profiles[j].Cost
	})

	// Combined savings
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

	// Combined Forecast
	report.CombinedForecast = calculateCombinedForecast(month, report.TotalCost, budgetCap)

	return report
}

func calculateCombinedForecast(month string, currentSpend, budgetCap float64) model.BurnRateForecast {
	now := time.Now()
	totalDays := 30
	activeDays := 1

	if t, err := time.Parse("2006-01", month); err == nil {
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

func handleSingleProfile(reader *db.Reader, allProfiles map[string]string, profileName, month string, outputFmt string, flags ui.RenderFlags) error {
	dbPath, exists := allProfiles[profileName]
	if !exists {
		return fmt.Errorf("profile '%s' not found or has no stats.db file", profileName)
	}

	if month == "all" {
		months, err := reader.GetDistinctMonths(dbPath)
		if err != nil || len(months) == 0 {
			fmt.Printf("\nℹ Profile '%s' has no recorded usage data.\n", profileName)
			return nil
		}
		for _, m := range months {
			summary, err := reader.BuildProfileSummary(profileName, dbPath, m, flags.BudgetCap)
			if err != nil {
				continue
			}
			report := &model.MultiProfileReport{
				Month:    m,
				Profiles: []*model.ProfileSummary{summary},
			}
			if err := outputReport(report, outputFmt, flags); err != nil {
				return err
			}
		}
		return nil
	}

	summary, err := reader.BuildProfileSummary(profileName, dbPath, month, flags.BudgetCap)
	if err != nil {
		return err
	}
	if summary.Requests == 0 {
		fmt.Printf("\nℹ No usage data found for profile '%s' in %s.\n", profileName, month)
		return nil
	}

	report := &model.MultiProfileReport{
		Month:    month,
		Profiles: []*model.ProfileSummary{summary},
	}
	return outputReport(report, outputFmt, flags)
}

func outputReport(report *model.MultiProfileReport, fmtType string, flags ui.RenderFlags) error {
	switch fmtType {
	case "json":
		s, err := export.ToJSON(report)
		if err != nil {
			return err
		}
		fmt.Println(s)
	case "csv":
		s, err := export.ToCSV(report.Profiles)
		if err != nil {
			return err
		}
		fmt.Print(s)
	case "md", "markdown":
		s := export.ToMarkdown(report)
		fmt.Println(s)
	default:
		if len(report.Profiles) == 1 {
			ui.RenderProfileSummary(report.Profiles[0], flags, false)
		} else {
			ui.RenderMultiProfileReport(report, flags)
		}
	}
	return nil
}
