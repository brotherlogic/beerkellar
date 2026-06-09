package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInitialTUIDashboardLayout(t *testing.T) {
	// Initialize a new default model
	model := initialModel(nil, nil)

	// Call View to get the rendered string
	rendered := model.View()

	// Assert that it contains all three pane headers and status line components
	expectedSections := []string{
		"CELLAR SUMMARY",
		"COMMAND READOUT",
		"COMMAND INPUT",
		"Untappd:",
		"Google Tasks:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(rendered, section) {
			t.Errorf("Expected TUI layout to contain %q, but got:\n%s", section, rendered)
		}
	}
}

func TestCommandInputWizardFlow(t *testing.T) {
	// Initialize a new default model
	model := initialModel(nil, nil)


	// Simulate entering 'add' to start the add beer wizard
	// Since tuiModel will need to handle message updates for key presses/text input,
	// let's send 'a', 'd', 'd', 'enter' as key messages.
	// We check if the model transitions to the wizard state and prompts for the beer name/ID.
	
	// We'll write the test using a mock update sequence.
	// Since the fields are not yet implemented on tuiModel, this test will fail to compile
	// or fail at assertion time, establishing the Red phase.
	m, _ := model.Update(mockKeyMsg("a"))
	m, _ = m.Update(mockKeyMsg("d"))
	m, _ = m.Update(mockKeyMsg("d"))
	m, _ = m.Update(mockKeyMsg("enter"))

	// View the model's command input pane / wizard prompt
	rendered := m.View()
	expectedPrompt := "Enter Beer ID"
	if !strings.Contains(rendered, expectedPrompt) {
		t.Errorf("Expected TUI to show wizard prompt %q, but got:\n%s", expectedPrompt, rendered)
	}
}

// Helper to simulate key presses in tests
type mockKey struct {
	runes []rune
	sym   string
}

func (m mockKey) String() string {
	if m.sym != "" {
		return m.sym
	}
	return string(m.runes)
}

func (m mockKey) Runes() []rune {
	return m.runes
}

func (m mockKey) Type() int {
	return 0
}

func mockKeyMsg(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{'\r'}}
	}
	return tea.KeyMsg{Runes: []rune(s)}
}

