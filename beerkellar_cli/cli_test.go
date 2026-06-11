package main

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/grpc"

	pb "github.com/brotherlogic/beerkellar/proto"
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

// mockBeerKellerClient implements pb.BeerKellerClient for testing
type mockBeerKellerClient struct {
	pb.BeerKellerClient
	cellarRes  *pb.GetCellarResponse
	weekdayRes *pb.GetBeerResponse
	weekendRes *pb.GetBeerResponse
	cellarErr  error
	weekdayErr error
	weekendErr error
}

func (m *mockBeerKellerClient) GetCellar(ctx context.Context, in *pb.GetCellarRequest, opts ...grpc.CallOption) (*pb.GetCellarResponse, error) {
	return m.cellarRes, m.cellarErr
}

func (m *mockBeerKellerClient) GetBeer(ctx context.Context, in *pb.GetBeerRequest, opts ...grpc.CallOption) (*pb.GetBeerResponse, error) {
	if len(in.Requirements) > 0 && in.Requirements[0].MaxUnits > 0 {
		return m.weekdayRes, m.weekdayErr
	}
	return m.weekendRes, m.weekendErr
}

func (m *mockBeerKellerClient) AddBeer(ctx context.Context, in *pb.AddBeerRequest, opts ...grpc.CallOption) (*pb.AddBeerResponse, error) {
	return &pb.AddBeerResponse{}, nil
}
func (m *mockBeerKellerClient) DrinkBeer(ctx context.Context, in *pb.DrinkBeerRequest, opts ...grpc.CallOption) (*pb.DrinkBeerResponse, error) {
	return &pb.DrinkBeerResponse{}, nil
}
func (m *mockBeerKellerClient) GetLogin(ctx context.Context, in *pb.GetLoginRequest, opts ...grpc.CallOption) (*pb.GetLoginResponse, error) {
	return &pb.GetLoginResponse{}, nil
}
func (m *mockBeerKellerClient) GetAuthToken(ctx context.Context, in *pb.GetAuthTokenRequest, opts ...grpc.CallOption) (*pb.GetAuthTokenResponse, error) {
	return &pb.GetAuthTokenResponse{}, nil
}
func (m *mockBeerKellerClient) GetDrunk(ctx context.Context, in *pb.GetDrunkRequest, opts ...grpc.CallOption) (*pb.GetDrunkResponse, error) {
	return &pb.GetDrunkResponse{}, nil
}
func (m *mockBeerKellerClient) Healthy(ctx context.Context, in *pb.HealthyRequest, opts ...grpc.CallOption) (*pb.HealthyResponse, error) {
	return &pb.HealthyResponse{}, nil
}

func TestCellarSummaryPane(t *testing.T) {
	mockClient := &mockBeerKellerClient{
		cellarRes: &pb.GetCellarResponse{
			Username: "testuser",
			State:    pb.User_STATE_LOGGED_IN,
			Beers: []*pb.Beer{
				{Id: 1, Name: "Beer 1", Brewery: "Brewery A", Units: 2.0}, // weekday (units < 2.5)
				{Id: 2, Name: "Beer 2", Brewery: "Brewery B", Units: 3.5}, // weekend
			},
		},
		weekdayRes: &pb.GetBeerResponse{
			Beers: []*pb.Beer{
				{Id: 1, Name: "Beer 1", Brewery: "Brewery A", Units: 2.0},
			},
		},
		weekendRes: &pb.GetBeerResponse{
			Beers: []*pb.Beer{
				{Id: 2, Name: "Beer 2", Brewery: "Brewery B", Units: 3.5},
			},
		},
	}

	model := initialModel(mockClient, nil)

	// In Bubble Tea, we trigger update with the message returned by the cmd.
	// Since we are in package main, we can call fetchCellarSummary() directly.
	msg := model.(tuiModel).fetchCellarSummary()()
	updatedModel, cmd := model.Update(msg)
	_ = cmd

	rendered := updatedModel.View()

	// Assertions for Cellar Summary content
	expectedStrings := []string{
		"2 Beers (1 Weekday, 1 Weekend)",
		"Next Weekday Candidate: Brewery A - Beer 1 (2.00 units)",
		"Next Weekend Candidate: Brewery B - Beer 2 (3.50 units)",
	}

	for _, s := range expectedStrings {
		if !strings.Contains(rendered, s) {
			t.Errorf("Expected summary pane to contain %q, but got:\n%s", s, rendered)
		}
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


