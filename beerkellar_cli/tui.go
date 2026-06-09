package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiModel struct {
	cellarSummary  string
	commandReadout string
	commandInput   string
	untappdStatus  string
	googleStatus   string
	err            error
}

func initialModel() tea.Model {
	return tuiModel{
		cellarSummary:  "CELLAR SUMMARY\nCellar Size & Split: 0 Beers (0 Weekday, 0 Weekend)\nNext Weekday Candidate: None\nNext Weekend Candidate: None",
		commandReadout: "COMMAND READOUT\nNo logs yet. Type a command below.",
		commandInput:   "COMMAND INPUT\n> ",
		untappdStatus:  "Untappd: Disconnected",
		googleStatus:   "Google Tasks: Disconnected",
	}
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m tuiModel) View() string {
	docStyle := lipgloss.NewStyle().Padding(1, 2)
	paneStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1)

	summaryView := paneStyle.Render(m.cellarSummary)
	readoutView := paneStyle.Render(m.commandReadout)
	inputView := paneStyle.Render(m.commandInput)

	footerView := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("245")).
		Render(fmt.Sprintf(" %s | %s ", m.untappdStatus, m.googleStatus))

	return docStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			summaryView,
			readoutView,
			inputView,
			footerView,
		),
	)
}
