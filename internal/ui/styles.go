package ui

import "github.com/charmbracelet/lipgloss"

// Styles for the chat interface
var (
	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	UserStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	AssistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB347")).
			Bold(true)

	MessageStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			MarginBottom(1)

	LoadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB347")).
			Italic(true)

	SidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("#444444"))

	SidebarFocusedStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("#25A065"))

	ChatStyle = lipgloss.NewStyle().
			PaddingLeft(1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)
)