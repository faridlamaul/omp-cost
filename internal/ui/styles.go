package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Catppuccin & Terminal Colors
var (
	ColorBlue    = lipgloss.Color("#89b4fa")
	ColorMauve   = lipgloss.Color("#cba6f7")
	ColorGreen   = lipgloss.Color("#a6e3a1")
	ColorYellow  = lipgloss.Color("#f9e2af")
	ColorPeach   = lipgloss.Color("#fab387")
	ColorSky     = lipgloss.Color("#89dceb")
	ColorRed     = lipgloss.Color("#f38ba8")
	ColorText    = lipgloss.Color("#cdd6f4")
	ColorSubtext = lipgloss.Color("#a6adc8")
	ColorLine    = lipgloss.Color("#585b70")
	ColorSurface = lipgloss.Color("#313244")
)

// Reusable Lipgloss Styles
var (
	StyleBold = lipgloss.NewStyle().Bold(true)
	StyleDim  = lipgloss.NewStyle().Faint(true)

	StyleHeaderTitle = lipgloss.NewStyle().Bold(true).Foreground(ColorBlue)
	StyleHeaderMeta  = lipgloss.NewStyle().Foreground(ColorSubtext)

	StyleCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorLine).
			Padding(0, 1)

	StyleTableHeader = lipgloss.NewStyle().Bold(true).Foreground(ColorText)
	StyleProvider    = lipgloss.NewStyle().Bold(true).Foreground(ColorMauve)
	StyleModel       = lipgloss.NewStyle().Foreground(ColorSky)
	StyleSubtotal    = lipgloss.NewStyle().Bold(true).Foreground(ColorYellow)
	StyleGrandTotal  = lipgloss.NewStyle().Bold(true).Foreground(ColorPeach)
	StyleCost        = lipgloss.NewStyle().Bold(true).Foreground(ColorGreen)
	StyleSecondary   = lipgloss.NewStyle().Foreground(ColorSubtext)
)

// FormatTokens formats token counts into readable K, M, B strings.
func FormatTokens(n int64) string {
	if n >= 1_000_000_000 {
		return fmt.Sprintf("%.2fB", float64(n)/1_000_000_000.0)
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000.0)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000.0)
	}
	return fmt.Sprintf("%d", n)
}

// FormatCost formats dollar amounts cleanly.
func FormatCost(val float64) string {
	if val >= 100.0 {
		return fmt.Sprintf("$%.2f", val)
	}
	if val >= 1.0 {
		return fmt.Sprintf("$%.3f", val)
	}
	if val > 0.0 {
		return fmt.Sprintf("$%.4f", val)
	}
	return "$0.00"
}

// MakeProgressBar renders a mini colored progress bar for cost percentage.
func MakeProgressBar(percent float64, width int) string {
	if width <= 0 {
		return ""
	}
	filled := int((percent / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled

	filledStr := lipgloss.NewStyle().Foreground(ColorGreen).Render(repeat("█", filled))
	emptyStr := lipgloss.NewStyle().Foreground(ColorLine).Render(repeat("░", empty))
	return filledStr + emptyStr
}

func repeat(s string, count int) string {
	res := ""
	for i := 0; i < count; i++ {
		res += s
	}
	return res
}
