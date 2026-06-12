package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	pb "github.com/brotherlogic/beerkellar/proto"
)

func TestInitialTUIDashboardLayout(t *testing.T) {
	// Initialize a new default model
	model := initialModel(nil, nil)

	// Call View to get the rendered string
	rendered := model.View()

	// Assert that it does NOT contain COMMAND READOUT initially
	if strings.Contains(rendered, "COMMAND READOUT") {
		t.Errorf("Expected initial TUI layout to NOT contain %q, but got:\n%s", "COMMAND READOUT", rendered)
	}

	// Assert that it contains other expected sections
	expectedSections := []string{
		"Untappd:",
		"Google Tasks:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(rendered, section) {
			t.Errorf("Expected TUI layout to contain %q, but got:\n%s", section, rendered)
		}
	}

	unexpectedSections := []string{
		"CELLAR SUMMARY",
		"COMMAND INPUT",
	}

	for _, section := range unexpectedSections {
		if strings.Contains(rendered, section) {
			t.Errorf("Expected TUI layout NOT to contain %q, but got:\n%s", section, rendered)
		}
	}

	if strings.Contains(rendered, "> >") {
		t.Errorf("Expected TUI layout not to contain double prompt '> >', but got:\n%s", rendered)
	}

	// Now set a command readout and verify it appears
	m := model.(tuiModel)
	m.commandReadout = "COMMAND READOUT\nSome command output"
	renderedWithReadout := m.View()
	if !strings.Contains(renderedWithReadout, "COMMAND READOUT") {
		t.Errorf("Expected TUI layout to contain %q after command output, but got:\n%s", "COMMAND READOUT", renderedWithReadout)
	}
}

func TestLogoRendering(t *testing.T) {
	model := initialModel(nil, nil)
	rendered := model.View()

	// We expect the ASCII logo containing "██████" to be rendered
	if !strings.Contains(rendered, "██████") {
		t.Errorf("Expected TUI layout to contain the stylized BEERKELLAR logo, but got:\n%s", rendered)
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

	if strings.Contains(rendered, "> >") {
		t.Errorf("Expected TUI layout in wizard mode not to contain double prompt '> >', but got:\n%s", rendered)
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

	getCellarFunc    func(ctx context.Context, in *pb.GetCellarRequest) (*pb.GetCellarResponse, error)
	getLoginFunc     func(ctx context.Context, in *pb.GetLoginRequest) (*pb.GetLoginResponse, error)
	getAuthTokenFunc func(ctx context.Context, in *pb.GetAuthTokenRequest) (*pb.GetAuthTokenResponse, error)
}

func (m *mockBeerKellerClient) GetCellar(ctx context.Context, in *pb.GetCellarRequest, opts ...grpc.CallOption) (*pb.GetCellarResponse, error) {
	if m.getCellarFunc != nil {
		return m.getCellarFunc(ctx, in)
	}
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
	if m.getLoginFunc != nil {
		return m.getLoginFunc(ctx, in)
	}
	return &pb.GetLoginResponse{}, nil
}
func (m *mockBeerKellerClient) GetAuthToken(ctx context.Context, in *pb.GetAuthTokenRequest, opts ...grpc.CallOption) (*pb.GetAuthTokenResponse, error) {
	if m.getAuthTokenFunc != nil {
		return m.getAuthTokenFunc(ctx, in)
	}
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

type mockGoogleClient struct {
	pb.BeerKellerGoogleClient
	getGoogleLoginFunc    func(ctx context.Context, in *pb.GetGoogleLoginRequest) (*pb.GetGoogleLoginResponse, error)
	toggleGoogleTasksFunc func(ctx context.Context, in *pb.ToggleGoogleTasksRequest) (*pb.ToggleGoogleTasksResponse, error)
}

func (m *mockGoogleClient) GetGoogleLogin(ctx context.Context, in *pb.GetGoogleLoginRequest, opts ...grpc.CallOption) (*pb.GetGoogleLoginResponse, error) {
	if m.getGoogleLoginFunc != nil {
		return m.getGoogleLoginFunc(ctx, in)
	}
	return &pb.GetGoogleLoginResponse{}, nil
}

func (m *mockGoogleClient) ToggleGoogleTasks(ctx context.Context, in *pb.ToggleGoogleTasksRequest, opts ...grpc.CallOption) (*pb.ToggleGoogleTasksResponse, error) {
	if m.toggleGoogleTasksFunc != nil {
		return m.toggleGoogleTasksFunc(ctx, in)
	}
	return &pb.ToggleGoogleTasksResponse{}, nil
}

func TestAsyncLoginFlow(t *testing.T) {
	// 1. Setup mock client
	mockClient := &mockBeerKellerClient{
		getLoginFunc: func(ctx context.Context, in *pb.GetLoginRequest) (*pb.GetLoginResponse, error) {
			return &pb.GetLoginResponse{
				Url:  "http://example.com/login",
				Code: "test_code_123",
			}, nil
		},
		getAuthTokenFunc: func(ctx context.Context, in *pb.GetAuthTokenRequest) (*pb.GetAuthTokenResponse, error) {
			if in.Code == "test_code_123" {
				return &pb.GetAuthTokenResponse{Code: "auth_token_456"}, nil
			}
			return nil, fmt.Errorf("invalid code")
		},
	}

	model := initialModel(mockClient, nil)

	// Simulate typing "login" and hitting enter
	m, cmd := model.Update(mockKeyMsg("l"))
	m, cmd = m.Update(mockKeyMsg("o"))
	m, cmd = m.Update(mockKeyMsg("g"))
	m, cmd = m.Update(mockKeyMsg("i"))
	m, cmd = m.Update(mockKeyMsg("n"))
	m, cmd = m.Update(mockKeyMsg("enter"))

	// Since login calls client.GetLogin asynchronously in a cmd, let's run the returned command
	if cmd == nil {
		t.Fatal("Expected cmd to start login but got nil")
	}
	
	msg := cmd()
	m, cmd = m.Update(msg)

	// Now check if a loginPollMsg/loginInitiatedMsg or next poll command was scheduled
	if cmd == nil {
		t.Fatal("Expected cmd for polling but got nil")
	}

	// Run the polling command
	msg = cmd()
	m, cmd = m.Update(msg)

	// Verify status line and readout has been updated on success
	rendered := m.View()
	if !strings.Contains(rendered, "Untappd: Logged In") {
		t.Errorf("Expected status line to contain 'Untappd: Logged In', but got:\n%s", rendered)
	}
}

func TestAsyncGoogleLoginFlow(t *testing.T) {
	// 1. Setup mock client
	mockGoogle := &mockGoogleClient{
		getGoogleLoginFunc: func(ctx context.Context, in *pb.GetGoogleLoginRequest) (*pb.GetGoogleLoginResponse, error) {
			return &pb.GetGoogleLoginResponse{
				Url: "http://example.com/google_login",
			}, nil
		},
		toggleGoogleTasksFunc: func(ctx context.Context, in *pb.ToggleGoogleTasksRequest) (*pb.ToggleGoogleTasksResponse, error) {
			return &pb.ToggleGoogleTasksResponse{}, nil
		},
	}

	model := initialModel(nil, mockGoogle)

	// Simulate typing "google login" and hitting enter
	m, cmd := model.Update(mockKeyMsg("g"))
	m, cmd = m.Update(mockKeyMsg("o"))
	m, cmd = m.Update(mockKeyMsg("o"))
	m, cmd = m.Update(mockKeyMsg("g"))
	m, cmd = m.Update(mockKeyMsg("l"))
	m, cmd = m.Update(mockKeyMsg("e"))
	m, cmd = m.Update(mockKeyMsg(" "))
	m, cmd = m.Update(mockKeyMsg("l"))
	m, cmd = m.Update(mockKeyMsg("o"))
	m, cmd = m.Update(mockKeyMsg("g"))
	m, cmd = m.Update(mockKeyMsg("i"))
	m, cmd = m.Update(mockKeyMsg("n"))
	m, cmd = m.Update(mockKeyMsg("enter"))

	if cmd == nil {
		t.Fatal("Expected cmd to start google login but got nil")
	}

	msg := cmd()
	m, cmd = m.Update(msg)

	if cmd == nil {
		t.Fatal("Expected cmd for polling google login but got nil")
	}

	// Run the polling command
	msg = cmd()
	m, cmd = m.Update(msg)

	// Verify status line has been updated on success
	rendered := m.View()
	if !strings.Contains(rendered, "Google Tasks: Linked") {
		t.Errorf("Expected status line to contain 'Google Tasks: Linked', but got:\n%s", rendered)
	}
}

func TestFetchCellarSummaryPropagatesToken(t *testing.T) {
	// Back up existing token file if it exists
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}
	tokenFile := filepath.Join(homeDir, ".beerkellar")
	backupFile := filepath.Join(homeDir, ".beerkellar.backup")

	if _, err := os.Stat(tokenFile); err == nil {
		if err := os.Rename(tokenFile, backupFile); err != nil {
			t.Fatalf("failed to backup token file: %v", err)
		}
		defer func() {
			if err := os.Rename(backupFile, tokenFile); err != nil {
				t.Errorf("failed to restore token file: %v", err)
			}
		}()
	} else {
		defer os.Remove(tokenFile)
	}

	// Write test token
	tokenData := []byte("code: \"test-token-123\"\n")
	if err := os.WriteFile(tokenFile, tokenData, 0600); err != nil {
		t.Fatalf("failed to write test token: %v", err)
	}

	var tokenFound string
	mockClient := &mockBeerKellerClient{
		getCellarFunc: func(ctx context.Context, in *pb.GetCellarRequest) (*pb.GetCellarResponse, error) {
			md, found := metadata.FromOutgoingContext(ctx)
			if found {
				if tokens, ok := md["auth-token"]; ok && len(tokens) > 0 {
					tokenFound = tokens[0]
				}
			}
			return &pb.GetCellarResponse{}, nil
		},
	}

	model := initialModel(mockClient, nil)
	_ = model.(tuiModel).fetchCellarSummary()()

	if tokenFound != "test-token-123" {
		t.Errorf("Expected token 'test-token-123' to be propagated, but got: %q", tokenFound)
	}
}

func TestTUIListCellarState(t *testing.T) {
	// Test case 1: State is STATE_AUTHORIZED - should not show state
	mockClientAuth := &mockBeerKellerClient{
		cellarRes: &pb.GetCellarResponse{
			Username: "testuser",
			State:    pb.User_STATE_AUTHORIZED,
		},
	}
	modelAuth := initialModel(mockClientAuth, nil).(tuiModel)
	msgAuth := modelAuth.runGetCellar()()
	resAuth, ok := msgAuth.(cmdResultMsg)
	if !ok {
		t.Fatalf("Expected cmdResultMsg, got %T", msgAuth)
	}
	if !strings.Contains(resAuth.content, "User: testuser") {
		t.Errorf("Expected output to contain 'User: testuser', got %q", resAuth.content)
	}
	if strings.Contains(resAuth.content, "STATE_AUTHORIZED") {
		t.Errorf("Expected output NOT to contain 'STATE_AUTHORIZED', got %q", resAuth.content)
	}

	// Test case 2: State is STATE_LOGGED_IN - should show state
	mockClientLoggedIn := &mockBeerKellerClient{
		cellarRes: &pb.GetCellarResponse{
			Username: "testuser",
			State:    pb.User_STATE_LOGGED_IN,
		},
	}
	modelLoggedIn := initialModel(mockClientLoggedIn, nil).(tuiModel)
	msgLoggedIn := modelLoggedIn.runGetCellar()()
	resLoggedIn, ok := msgLoggedIn.(cmdResultMsg)
	if !ok {
		t.Fatalf("Expected cmdResultMsg, got %T", msgLoggedIn)
	}
	if !strings.Contains(resLoggedIn.content, "User: testuser (State: STATE_LOGGED_IN)") {
		t.Errorf("Expected output to contain 'User: testuser (State: STATE_LOGGED_IN)', got %q", resLoggedIn.content)
	}
}

func TestTUIBoxWidthOnResize(t *testing.T) {
	model := initialModel(nil, nil)

	// Send a WindowSizeMsg to set width
	m, cmd := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Errorf("Expected nil command, got %v", cmd)
	}

	rendered := m.View()

	// Check that the rendered view contains lines with the expected width.
	// W = 80, minus docStyle horizontal padding (4) => pane width = 76.
	// The top border of pane width 76 will be "┌" + 74 "─" + "┐".
	expectedBorder := "┌" + strings.Repeat("─", 74) + "┐"
	if !strings.Contains(rendered, expectedBorder) {
		t.Errorf("Expected rendered view to contain box border %q, but got:\n%s", expectedBorder, rendered)
	}
}



