package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	// Status colors: green = ok final, yellow = in progress, red = error
	greenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("34"))

	yellowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	redStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// Untouched resources: plain grey
	untouchedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	// Deleted resources: dim dark red
	deletedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("88"))

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

// statusStyle returns the appropriate style for a stack or resource status.
// Green: final success (CREATE_COMPLETE, UPDATE_COMPLETE, DELETE_COMPLETE, IMPORT_COMPLETE)
// Yellow: in-progress states
// Red: anything with FAILED or ROLLBACK
func statusStyle(status string) lipgloss.Style {
	if strings.Contains(status, "FAILED") {
		return redStyle
	}
	if strings.Contains(status, "ROLLBACK") {
		return redStyle
	}
	if isActive(status) {
		return yellowStyle
	}
	return greenStyle
}
