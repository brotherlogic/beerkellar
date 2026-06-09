package main

import (
	"strings"
	"testing"
)

func TestInitialTUIDashboardLayout(t *testing.T) {
	// Initialize a new default model
	model := initialModel()

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
