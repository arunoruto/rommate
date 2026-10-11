package tui

import "github.com/charmbracelet/lipgloss"

// Palette. Adaptive colors keep the UI readable on light and dark terminals.
var (
	colAccent   = lipgloss.AdaptiveColor{Light: "#5A3FD9", Dark: "#A08CFF"}
	colAccent2  = lipgloss.AdaptiveColor{Light: "#C2367A", Dark: "#FF7AB8"}
	colText     = lipgloss.AdaptiveColor{Light: "#1F1F28", Dark: "#E6E6F0"}
	colSubtle   = lipgloss.AdaptiveColor{Light: "#8A8A99", Dark: "#7C7C8C"}
	colFaint    = lipgloss.AdaptiveColor{Light: "#D4D4DE", Dark: "#3A3A48"}
	colGood     = lipgloss.AdaptiveColor{Light: "#1E8A3A", Dark: "#5FD37A"}
	colWarn     = lipgloss.AdaptiveColor{Light: "#B7791F", Dark: "#F6C177"}
	colBad      = lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#FF6B6B"}
	colOnAccent = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#16161E"}
)

var (
	sText   = lipgloss.NewStyle().Foreground(colText)
	sDim    = lipgloss.NewStyle().Foreground(colSubtle)
	sFaint  = lipgloss.NewStyle().Foreground(colFaint)
	sBold   = lipgloss.NewStyle().Foreground(colText).Bold(true)
	sAccent = lipgloss.NewStyle().Foreground(colAccent)
	sGood   = lipgloss.NewStyle().Foreground(colGood)
	sWarn   = lipgloss.NewStyle().Foreground(colWarn)
	sBad    = lipgloss.NewStyle().Foreground(colBad)

	// Header "pill" with the app name.
	sLogo = lipgloss.NewStyle().Bold(true).Foreground(colOnAccent).Background(colAccent).Padding(0, 1)

	// Header tabs.
	sTab       = lipgloss.NewStyle().Foreground(colSubtle).Padding(0, 1)
	sTabActive = lipgloss.NewStyle().Bold(true).Foreground(colText).Underline(true).Padding(0, 1)

	// Selected list row.
	sSelected = lipgloss.NewStyle().Bold(true).Foreground(colOnAccent).Background(colAccent)

	// Footer button caps, e.g. [ A ] Download.
	sKeyCap   = lipgloss.NewStyle().Bold(true).Foreground(colOnAccent).Background(colAccent2).Padding(0, 1)
	sKeyLabel = lipgloss.NewStyle().Foreground(colSubtle)
)
