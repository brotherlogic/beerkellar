package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
		"Version: unknown",
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
	removeBeerFunc   func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error)
}

func (m *mockBeerKellerClient) GetCellar(ctx context.Context, in *pb.GetCellarRequest, opts ...grpc.CallOption) (*pb.GetCellarResponse, error) {
	if m.getCellarFunc != nil {
		return m.getCellarFunc(ctx, in)
	}
	return m.cellarRes, m.cellarErr
}

func (m *mockBeerKellerClient) GetBeer(ctx context.Context, in *pb.GetBeerRequest, opts ...grpc.CallOption) (*pb.GetBeerResponse, error) {
	if len(in.Requirements) > 0 && in.Requirements[0].MaxUnits > 0 {
		if in.Requirements[0].MaxUnits != weekdayBeerUnitsLimit {
			return nil, fmt.Errorf("expected MaxUnits %v, got %v", weekdayBeerUnitsLimit, in.Requirements[0].MaxUnits)
		}
		return m.weekdayRes, m.weekdayErr
	}
	if len(in.Requirements) > 0 && in.Requirements[0].MinUnits > 0 {
		if in.Requirements[0].MinUnits != weekdayBeerUnitsLimit {
			return nil, fmt.Errorf("expected MinUnits %v, got %v", weekdayBeerUnitsLimit, in.Requirements[0].MinUnits)
		}
		return m.weekendRes, m.weekendErr
	}
	return m.weekendRes, m.weekendErr
}

func (m *mockBeerKellerClient) AddBeer(ctx context.Context, in *pb.AddBeerRequest, opts ...grpc.CallOption) (*pb.AddBeerResponse, error) {
	return &pb.AddBeerResponse{}, nil
}
func (m *mockBeerKellerClient) DrinkBeer(ctx context.Context, in *pb.DrinkBeerRequest, opts ...grpc.CallOption) (*pb.DrinkBeerResponse, error) {
	return &pb.DrinkBeerResponse{}, nil
}
func (m *mockBeerKellerClient) RemoveBeer(ctx context.Context, in *pb.RemoveBeerRequest, opts ...grpc.CallOption) (*pb.RemoveBeerResponse, error) {
	if m.removeBeerFunc != nil {
		return m.removeBeerFunc(ctx, in)
	}
	return &pb.RemoveBeerResponse{}, nil
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
				{Id: 1, Name: "Beer 1", Brewery: "Brewery A", Units: 2.0}, // weekday (units < 3.5)
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
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{'\r'}}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Runes: []rune(s)}
	}
}

func typeCommand(m tea.Model, text string) (tea.Model, tea.Cmd) {
	for _, r := range text {
		m, _ = m.Update(mockKeyMsg(string(r)))
	}
	return m.Update(mockKeyMsg("enter"))
}

func mustRunCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("Expected non-nil tea.Cmd, got nil")
	}
	return cmd()
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

func TestAutoUpdateTUIDevMode(t *testing.T) {
	// Preserve original version
	origVersion := Version
	defer func() { Version = origVersion }()

	Version = "unknown"
	
	// This should return immediately without doing any network calls or process replacement
	autoUpdateTUI()
}

func sampleRemoveCellarResponse() *pb.GetCellarResponse {
	return &pb.GetCellarResponse{
		Username: "testuser",
		State:    pb.User_STATE_AUTHORIZED,
		Beers: []*pb.Beer{
			{Id: 101, Name: "Beer 1", Brewery: "Brewery A", Units: 2.0},
			{Id: 202, Name: "Beer 2", Brewery: "Brewery B", Units: 4.0},
		},
		Entries: []*pb.CellarEntry{
			{BeerId: 101, DateAdded: 1700000100, SizeFlOz: 12},
			{BeerId: 202, DateAdded: 1700000200, SizeFlOz: 16},
		},
	}
}

func TestRemoveWizardGuidedFlow(t *testing.T) {
	cases := []struct {
		name        string
		removeCmd   string
		confirmText string
	}{
		{name: "lowercase_y", removeCmd: "remove", confirmText: "y"},
		{name: "lowercase_yes", removeCmd: "remove", confirmText: "yes"},
		{name: "uppercase_trimmed", removeCmd: "  REMOVE  ", confirmText: "  YES  "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var removedReq *pb.RemoveBeerRequest
			removeCalls := 0
			cellarCalls := 0

			mockClient := &mockBeerKellerClient{
				getCellarFunc: func(ctx context.Context, in *pb.GetCellarRequest) (*pb.GetCellarResponse, error) {
					cellarCalls++
					if cellarCalls == 1 {
						return sampleRemoveCellarResponse(), nil
					}
					// Summary refresh after removal returns remaining 1 beer
					return &pb.GetCellarResponse{
						Username: "testuser",
						State:    pb.User_STATE_AUTHORIZED,
						Beers: []*pb.Beer{
							{Id: 101, Name: "Beer 1", Brewery: "Brewery A", Units: 2.0},
						},
						Entries: []*pb.CellarEntry{
							{BeerId: 101, DateAdded: 1700000100, SizeFlOz: 12},
						},
					}, nil
				},
				removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
					removeCalls++
					removedReq = in
					return &pb.RemoveBeerResponse{}, nil
				},
				weekdayRes: &pb.GetBeerResponse{
					Beers: []*pb.Beer{{Id: 101, Name: "Beer 1", Brewery: "Brewery A", Units: 2.0}},
				},
				weekendRes: &pb.GetBeerResponse{},
			}

			m := initialModel(mockClient, nil)

			// Step 1: Enter remove command
			m, cmd := typeCommand(m, tc.removeCmd)
			if cmd == nil {
				t.Fatal("Expected fetch cellar cmd after 'remove', got nil")
			}
			tm := m.(tuiModel)
			if !strings.Contains(tm.commandReadout, "Fetching cellar...") {
				t.Errorf("Expected 'Fetching cellar...' in readout, got %q", tm.commandReadout)
			}
			if tm.textInput.Placeholder != "Loading cellar..." {
				t.Errorf("Expected placeholder 'Loading cellar...', got %q", tm.textInput.Placeholder)
			}

			// Deliver cellar loaded message
			m, cmd = m.Update(cmd())
			if cmd != nil {
				t.Errorf("Expected nil cmd after loading cellar list, got %v", cmd)
			}
			tm = m.(tuiModel)
			for _, want := range []string{
				"1. Brewery A - Beer 1 (2.00 units)",
				"2. Brewery B - Beer 2 (4.00 units)",
			} {
				if !strings.Contains(tm.commandReadout, want) {
					t.Errorf("Expected readout to contain %q, got:\n%s", want, tm.commandReadout)
				}
			}
			if tm.textInput.Placeholder != "Enter item number (1-2) or 'cancel'" {
				t.Errorf("Expected guided placeholder, got %q", tm.textInput.Placeholder)
			}

			// Step 2: Select item #2
			m, cmd = typeCommand(m, "2")
			if cmd != nil {
				t.Errorf("Expected nil cmd after selecting item, got %v", cmd)
			}
			tm = m.(tuiModel)
			expectedConfirm := `Remove #2 "Brewery B - Beer 2 (4.00 units)"? (y/n)`
			if !strings.Contains(tm.commandReadout, expectedConfirm) {
				t.Errorf("Expected confirmation prompt %q, got:\n%s", expectedConfirm, tm.commandReadout)
			}
			if tm.textInput.Placeholder != "Confirm removal (y/n)" {
				t.Errorf("Expected placeholder 'Confirm removal (y/n)', got %q", tm.textInput.Placeholder)
			}
			if len(tm.textInput.Placeholder) > 40 {
				t.Errorf("Expected placeholder to fit within 40 chars, got %d (%q)", len(tm.textInput.Placeholder), tm.textInput.Placeholder)
			}

			// Step 3: Confirm removal
			m, cmd = typeCommand(m, tc.confirmText)
			if cmd == nil {
				t.Fatal("Expected RemoveBeer cmd after confirmation, got nil")
			}
			if m.(tuiModel).textInput.Placeholder != "Removing beer..." {
				t.Errorf("Expected placeholder 'Removing beer...', got %q", m.(tuiModel).textInput.Placeholder)
			}

			// Execute RemoveBeer cmd and deliver result
			m, summaryCmd := m.Update(cmd())
			if removeCalls != 1 {
				t.Fatalf("Expected RemoveBeer to be called once, got %d", removeCalls)
			}
			if removedReq.GetIndex() != 1 || removedReq.GetBeerId() != 202 || removedReq.GetSizeFlOz() != 16 || removedReq.GetDateAdded() != 1700000200 {
				t.Errorf("Unexpected RemoveBeerRequest: %+v", removedReq)
			}

			tm = m.(tuiModel)
			expectedSuccess := `Removed "Brewery B - Beer 2 (4.00 units)" from cellar.`
			if !strings.Contains(tm.commandReadout, expectedSuccess) {
				t.Errorf("Expected readout %q, got:\n%s", expectedSuccess, tm.commandReadout)
			}
			if tm.textInput.Placeholder != "Type command here..." {
				t.Errorf("Expected reset placeholder 'Type command here...', got %q", tm.textInput.Placeholder)
			}
			if tm.activeWiz != wizardNone {
				t.Errorf("Expected activeWiz reset to wizardNone, got %v", tm.activeWiz)
			}
			if summaryCmd == nil {
				t.Fatal("Expected fetchCellarSummary cmd after successful removal, got nil")
			}

			// Run summary refresh cmd and verify top summary updates
			m, _ = m.Update(summaryCmd())
			rendered := m.View()
			if !strings.Contains(rendered, "1 Beers (1 Weekday, 0 Weekend)") {
				t.Errorf("Expected refreshed summary pane to show 1 Beer, got:\n%s", rendered)
			}
		})
	}
}

func TestRemoveWizardDirectShortcut(t *testing.T) {
	var removedReq *pb.RemoveBeerRequest
	removeCalls := 0
	cellarCalls := 0

	mockClient := &mockBeerKellerClient{
		getCellarFunc: func(ctx context.Context, in *pb.GetCellarRequest) (*pb.GetCellarResponse, error) {
			cellarCalls++
			if cellarCalls == 1 {
				return sampleRemoveCellarResponse(), nil
			}
			return &pb.GetCellarResponse{
				Beers:   []*pb.Beer{{Id: 101, Name: "Beer 1", Brewery: "Brewery A", Units: 2.0}},
				Entries: []*pb.CellarEntry{{BeerId: 101, DateAdded: 1700000100, SizeFlOz: 12}},
			}, nil
		},
		removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
			removeCalls++
			removedReq = in
			return &pb.RemoveBeerResponse{}, nil
		},
	}

	m := initialModel(mockClient, nil)
	m, cmd := typeCommand(m, "  REMOVE   2  ")
	if cmd == nil {
		t.Fatal("Expected fetch cmd for shortcut 'remove 2', got nil")
	}

	// Deliver cellar fetch -> should skip numbered list and go straight to confirmation
	m, cmd = m.Update(cmd())
	if cmd != nil {
		t.Errorf("Expected nil cmd after shortcut fetch, got %v", cmd)
	}
	tm := m.(tuiModel)
	if strings.Contains(tm.commandReadout, "1. Brewery A") {
		t.Errorf("Expected shortcut flow to skip numbered list, got:\n%s", tm.commandReadout)
	}
	expectedConfirm := `Remove #2 "Brewery B - Beer 2 (4.00 units)"? (y/n)`
	if !strings.Contains(tm.commandReadout, expectedConfirm) {
		t.Errorf("Expected confirmation %q, got:\n%s", expectedConfirm, tm.commandReadout)
	}
	if tm.textInput.Placeholder != "Confirm removal (y/n)" {
		t.Errorf("Expected placeholder 'Confirm removal (y/n)', got %q", tm.textInput.Placeholder)
	}

	m, cmd = typeCommand(m, "y")
	if cmd == nil {
		t.Fatal("Expected RemoveBeer cmd, got nil")
	}
	m, summaryCmd := m.Update(cmd())
	if removeCalls != 1 {
		t.Fatalf("Expected 1 RemoveBeer call, got %d", removeCalls)
	}
	if removedReq.GetIndex() != 1 || removedReq.GetBeerId() != 202 || removedReq.GetSizeFlOz() != 16 || removedReq.GetDateAdded() != 1700000200 {
		t.Errorf("Unexpected RemoveBeerRequest: %+v", removedReq)
	}
	if !strings.Contains(m.(tuiModel).commandReadout, `Removed "Brewery B - Beer 2 (4.00 units)" from cellar.`) {
		t.Errorf("Unexpected success readout: %s", m.(tuiModel).commandReadout)
	}
	if summaryCmd == nil {
		t.Fatal("Expected summary refresh cmd after removal, got nil")
	}
}

func TestRemoveWizardCancellation(t *testing.T) {
	t.Run("text_cancel_at_select_index", func(t *testing.T) {
		for _, cancelWord := range []string{"n", "no", "cancel", "Cancel", "NO"} {
			t.Run(cancelWord, func(t *testing.T) {
				removeCalls := 0
				mockClient := &mockBeerKellerClient{
					cellarRes: sampleRemoveCellarResponse(),
					removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
						removeCalls++
						return &pb.RemoveBeerResponse{}, nil
					},
				}
				m := initialModel(mockClient, nil)
				m, cmd := typeCommand(m, "remove")
				m, _ = m.Update(mustRunCmd(t, cmd))

				m, cmd = typeCommand(m, cancelWord)
				if cmd != nil {
					t.Errorf("Expected nil cmd on cancel, got %v", cmd)
				}
				if removeCalls != 0 {
					t.Errorf("Expected 0 RemoveBeer calls, got %d", removeCalls)
				}
				tm := m.(tuiModel)
				if tm.activeWiz != wizardNone {
					t.Errorf("Expected wizard reset to wizardNone, got %v", tm.activeWiz)
				}
				if tm.textInput.Placeholder != "Type command here..." {
					t.Errorf("Expected default placeholder, got %q", tm.textInput.Placeholder)
				}
				if !strings.Contains(tm.commandReadout, "Removal cancelled. No beer was removed.") {
					t.Errorf("Expected cancellation readout, got %q", tm.commandReadout)
				}
			})
		}
	})

	t.Run("text_cancel_at_confirm", func(t *testing.T) {
		for _, cancelWord := range []string{"n", "no", "cancel", "CANCEL"} {
			t.Run(cancelWord, func(t *testing.T) {
				removeCalls := 0
				mockClient := &mockBeerKellerClient{
					cellarRes: sampleRemoveCellarResponse(),
					removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
						removeCalls++
						return &pb.RemoveBeerResponse{}, nil
					},
				}
				m := initialModel(mockClient, nil)
				m, cmd := typeCommand(m, "remove 1")
				m, _ = m.Update(mustRunCmd(t, cmd))

				m, cmd = typeCommand(m, cancelWord)
				if cmd != nil {
					t.Errorf("Expected nil cmd on cancel, got %v", cmd)
				}
				if removeCalls != 0 {
					t.Errorf("Expected 0 RemoveBeer calls, got %d", removeCalls)
				}
				tm := m.(tuiModel)
				if tm.activeWiz != wizardNone || tm.textInput.Placeholder != "Type command here..." {
					t.Errorf("Expected wizard reset, got activeWiz=%v placeholder=%q", tm.activeWiz, tm.textInput.Placeholder)
				}
				if !strings.Contains(tm.commandReadout, "Removal cancelled. No beer was removed.") {
					t.Errorf("Expected cancellation readout, got %q", tm.commandReadout)
				}
			})
		}
	})

	t.Run("esc_during_fetching_ignores_stale_response", func(t *testing.T) {
		mockClient := &mockBeerKellerClient{
			cellarRes: sampleRemoveCellarResponse(),
		}
		m := initialModel(mockClient, nil)

		// Start first remove fetch
		m, fetchCmd1 := typeCommand(m, "remove")
		if fetchCmd1 == nil {
			t.Fatal("Expected fetchCmd1, got nil")
		}

		// Press Esc before async fetch completes
		m, escCmd := m.Update(mockKeyMsg("esc"))
		if escCmd != nil {
			t.Errorf("Expected nil cmd on Esc, got %v", escCmd)
		}
		tm := m.(tuiModel)
		if tm.activeWiz != wizardNone {
			t.Errorf("Expected wizardNone after Esc, got %v", tm.activeWiz)
		}
		if !strings.Contains(tm.commandReadout, "Removal cancelled. No beer was removed.") {
			t.Errorf("Expected cancellation message, got %q", tm.commandReadout)
		}

		// Now stale fetch response arrives -> must be ignored
		staleMsg1 := fetchCmd1()
		m, _ = m.Update(staleMsg1)
		tm = m.(tuiModel)
		if tm.activeWiz != wizardNone {
			t.Errorf("Expected stale response to be ignored and remain wizardNone, got %v", tm.activeWiz)
		}
		if !strings.Contains(tm.commandReadout, "Removal cancelled. No beer was removed.") {
			t.Errorf("Expected cancelled readout to remain intact, got %q", tm.commandReadout)
		}

		// Also verify starting a second remove ignores staleMsg1 while accepting fetchCmd2
		m, fetchCmd2 := typeCommand(m, "remove 1")
		if fetchCmd2 == nil {
			t.Fatal("Expected fetchCmd2, got nil")
		}
		// Re-deliver staleMsg1 during active fetch #2 -> must be ignored
		m, _ = m.Update(staleMsg1)
		if !strings.Contains(m.(tuiModel).commandReadout, "Fetching cellar...") {
			t.Errorf("Expected stale fetch #1 to be ignored during fetch #2, got %q", m.(tuiModel).commandReadout)
		}
		// Deliver fetchCmd2 -> must transition to confirm for #1
		m, _ = m.Update(fetchCmd2())
		if !strings.Contains(m.(tuiModel).commandReadout, `Remove #1 "Brewery A - Beer 1 (2.00 units)"? (y/n)`) {
			t.Errorf("Expected fetch #2 to succeed, got %q", m.(tuiModel).commandReadout)
		}
	})

	t.Run("esc_during_select_and_confirm", func(t *testing.T) {
		removeCalls := 0
		mockClient := &mockBeerKellerClient{
			cellarRes: sampleRemoveCellarResponse(),
			removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
				removeCalls++
				return &pb.RemoveBeerResponse{}, nil
			},
		}

		// Esc at select index
		m := initialModel(mockClient, nil)
		m, cmd := typeCommand(m, "remove")
		m, _ = m.Update(mustRunCmd(t, cmd))
		m, _ = m.Update(mockKeyMsg("esc"))
		if m.(tuiModel).activeWiz != wizardNone || !strings.Contains(m.(tuiModel).commandReadout, "Removal cancelled. No beer was removed.") {
			t.Errorf("Expected Esc to cancel from select step, got activeWiz=%v readout=%q", m.(tuiModel).activeWiz, m.(tuiModel).commandReadout)
		}

		// Esc at confirm
		m, cmd = typeCommand(m, "remove 1")
		m, _ = m.Update(mustRunCmd(t, cmd))
		m, _ = m.Update(mockKeyMsg("esc"))
		if m.(tuiModel).activeWiz != wizardNone || !strings.Contains(m.(tuiModel).commandReadout, "Removal cancelled. No beer was removed.") {
			t.Errorf("Expected Esc to cancel from confirm step, got activeWiz=%v readout=%q", m.(tuiModel).activeWiz, m.(tuiModel).commandReadout)
		}
		if removeCalls != 0 {
			t.Errorf("Expected 0 RemoveBeer calls, got %d", removeCalls)
		}
	})

	t.Run("esc_during_removing_does_not_claim_no_beer_removed", func(t *testing.T) {
		mockClient := &mockBeerKellerClient{
			cellarRes: sampleRemoveCellarResponse(),
		}
		m := initialModel(mockClient, nil)
		m, cmd := typeCommand(m, "remove 1")
		m, _ = m.Update(mustRunCmd(t, cmd))
		m, removeCmd := typeCommand(m, "y")
		if removeCmd == nil {
			t.Fatal("Expected removeCmd, got nil")
		}

		m, _ = m.Update(mockKeyMsg("esc"))
		if strings.Contains(m.(tuiModel).commandReadout, "No beer was removed") {
			t.Errorf("Expected Esc during removing RPC NOT to claim no beer was removed, got %q", m.(tuiModel).commandReadout)
		}
		if m.(tuiModel).activeWiz == wizardNone {
			t.Errorf("Expected Esc during removing RPC to leave wizard in removing state")
		}
	})

	t.Run("ctrl_c_exits_cleanly_from_all_wizard_steps", func(t *testing.T) {
		removeCalls := 0
		mockClient := &mockBeerKellerClient{
			cellarRes: sampleRemoveCellarResponse(),
			removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
				removeCalls++
				return &pb.RemoveBeerResponse{}, nil
			},
		}

		// 1. During fetching
		m1, _ := typeCommand(initialModel(mockClient, nil), "remove")
		_, quitCmd1 := m1.Update(mockKeyMsg("ctrl+c"))
		if quitCmd1 == nil {
			t.Fatal("Expected tea.Quit cmd on Ctrl+C during fetching")
		}
		if _, ok := quitCmd1().(tea.QuitMsg); !ok {
			t.Errorf("Expected tea.QuitMsg, got %T", quitCmd1())
		}

		// 2. During select index
		m2, fetchCmd := typeCommand(initialModel(mockClient, nil), "remove")
		m2, _ = m2.Update(mustRunCmd(t, fetchCmd))
		_, quitCmd2 := m2.Update(mockKeyMsg("ctrl+c"))
		if quitCmd2 == nil {
			t.Fatal("Expected tea.Quit cmd on Ctrl+C during select index")
		}
		if _, ok := quitCmd2().(tea.QuitMsg); !ok {
			t.Errorf("Expected tea.QuitMsg, got %T", quitCmd2())
		}

		// 3. During confirm
		m3, fetchCmd3 := typeCommand(initialModel(mockClient, nil), "remove 1")
		m3, _ = m3.Update(mustRunCmd(t, fetchCmd3))
		_, quitCmd3 := m3.Update(mockKeyMsg("ctrl+c"))
		if quitCmd3 == nil {
			t.Fatal("Expected tea.Quit cmd on Ctrl+C during confirm")
		}
		if _, ok := quitCmd3().(tea.QuitMsg); !ok {
			t.Errorf("Expected tea.QuitMsg, got %T", quitCmd3())
		}

		if removeCalls != 0 {
			t.Errorf("Expected 0 RemoveBeer calls on Ctrl+C, got %d", removeCalls)
		}
	})
}

func TestRemoveWizardUnrecognizedConfirmation(t *testing.T) {
	removeCalls := 0
	mockClient := &mockBeerKellerClient{
		cellarRes: sampleRemoveCellarResponse(),
		removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
			removeCalls++
			return &pb.RemoveBeerResponse{}, nil
		},
	}

	m := initialModel(mockClient, nil)
	m, cmd := typeCommand(m, "remove 1")
	m, _ = m.Update(mustRunCmd(t, cmd))

	for _, invalid := range []string{"maybe", "", "1", "yep"} {
		m, cmd = typeCommand(m, invalid)
		if cmd != nil {
			t.Errorf("Expected nil cmd for unrecognized confirmation %q, got %v", invalid, cmd)
		}
		if removeCalls != 0 {
			t.Errorf("Expected 0 RemoveBeer calls after %q, got %d", invalid, removeCalls)
		}
		tm := m.(tuiModel)
		if tm.activeWiz == wizardNone {
			t.Errorf("Expected wizard to remain active after unrecognized confirmation %q", invalid)
		}
		wantPrompt := `Please enter 'y' to confirm removal of #1 "Brewery A - Beer 1 (2.00 units)" or 'n' to cancel.`
		if !strings.Contains(tm.commandReadout, wantPrompt) {
			t.Errorf("Expected re-prompt %q, got:\n%s", wantPrompt, tm.commandReadout)
		}
	}

	// Confirming with 'y' afterward still succeeds
	m, cmd = typeCommand(m, "y")
	if cmd == nil {
		t.Fatal("Expected RemoveBeer cmd after valid 'y'")
	}
	m, _ = m.Update(cmd())
	if removeCalls != 1 {
		t.Errorf("Expected 1 RemoveBeer call, got %d", removeCalls)
	}
	if !strings.Contains(m.(tuiModel).commandReadout, `Removed "Brewery A - Beer 1 (2.00 units)" from cellar.`) {
		t.Errorf("Expected removal success readout, got %q", m.(tuiModel).commandReadout)
	}
}

func TestRemoveWizardValidation(t *testing.T) {
	t.Run("empty_cellar_guided_and_shortcut", func(t *testing.T) {
		for _, command := range []string{"remove", "remove 1"} {
			t.Run(command, func(t *testing.T) {
				mockClient := &mockBeerKellerClient{
					cellarRes: &pb.GetCellarResponse{Username: "testuser", State: pb.User_STATE_AUTHORIZED},
				}
				m := initialModel(mockClient, nil)
				m, cmd := typeCommand(m, command)
				m, _ = m.Update(mustRunCmd(t, cmd))

				tm := m.(tuiModel)
				if tm.activeWiz != wizardNone || tm.textInput.Placeholder != "Type command here..." {
					t.Errorf("Expected wizard reset on empty cellar, got activeWiz=%v placeholder=%q", tm.activeWiz, tm.textInput.Placeholder)
				}
				if !strings.Contains(tm.commandReadout, "Your cellar is empty. Nothing to remove.") {
					t.Errorf("Expected empty cellar message, got %q", tm.commandReadout)
				}
			})
		}
	})

	t.Run("mismatched_entries_length", func(t *testing.T) {
		mockClient := &mockBeerKellerClient{
			cellarRes: &pb.GetCellarResponse{
				Beers:   []*pb.Beer{{Id: 101, Name: "Beer 1", Brewery: "Brewery A", Units: 2.0}},
				Entries: nil,
			},
		}
		m := initialModel(mockClient, nil)
		m, cmd := typeCommand(m, "remove")
		m, _ = m.Update(mustRunCmd(t, cmd))

		tm := m.(tuiModel)
		if tm.activeWiz != wizardNone {
			t.Errorf("Expected wizard reset on entries mismatch, got %v", tm.activeWiz)
		}
		if !strings.Contains(strings.ToLower(tm.commandReadout), "mismatch") {
			t.Errorf("Expected entries mismatch error, got %q", tm.commandReadout)
		}
	})

	t.Run("extra_args", func(t *testing.T) {
		getCellarCalls := 0
		mockClient := &mockBeerKellerClient{
			getCellarFunc: func(ctx context.Context, in *pb.GetCellarRequest) (*pb.GetCellarResponse, error) {
				getCellarCalls++
				return sampleRemoveCellarResponse(), nil
			},
		}
		for _, command := range []string{"remove 1 2", "remove 1 foo"} {
			m := initialModel(mockClient, nil)
			m, cmd := typeCommand(m, command)
			if cmd != nil {
				t.Errorf("Expected nil cmd for %q, got %v", command, cmd)
			}
			tm := m.(tuiModel)
			if tm.activeWiz != wizardNone {
				t.Errorf("Expected wizardNone for %q, got %v", command, tm.activeWiz)
			}
			if !strings.Contains(tm.commandReadout, "Usage: remove [item number]") {
				t.Errorf("Expected usage message for %q, got %q", command, tm.commandReadout)
			}
		}
		if getCellarCalls != 0 {
			t.Errorf("Expected 0 GetCellar calls on invalid syntax, got %d", getCellarCalls)
		}
	})

	t.Run("invalid_shortcut_numbers", func(t *testing.T) {
		getCellarCalls := 0
		mockClient := &mockBeerKellerClient{
			getCellarFunc: func(ctx context.Context, in *pb.GetCellarRequest) (*pb.GetCellarResponse, error) {
				getCellarCalls++
				return sampleRemoveCellarResponse(), nil
			},
		}
		for _, command := range []string{"remove abc", "remove 0", "remove -1", "remove 99999999999999999999999"} {
			m := initialModel(mockClient, nil)
			m, cmd := typeCommand(m, command)
			if cmd != nil {
				t.Errorf("Expected nil cmd for %q, got %v", command, cmd)
			}
			tm := m.(tuiModel)
			if tm.activeWiz != wizardNone {
				t.Errorf("Expected wizardNone for %q, got %v", command, tm.activeWiz)
			}
			if !strings.Contains(tm.commandReadout, "Invalid item number") {
				t.Errorf("Expected invalid item number error for %q, got %q", command, tm.commandReadout)
			}
		}
		if getCellarCalls != 0 {
			t.Errorf("Expected 0 GetCellar calls on invalid shortcut arg, got %d", getCellarCalls)
		}
	})

	t.Run("out_of_range_shortcut", func(t *testing.T) {
		mockClient := &mockBeerKellerClient{
			cellarRes: sampleRemoveCellarResponse(),
		}
		m := initialModel(mockClient, nil)
		m, cmd := typeCommand(m, "remove 5")
		m, _ = m.Update(mustRunCmd(t, cmd))

		tm := m.(tuiModel)
		if tm.activeWiz != wizardNone || tm.textInput.Placeholder != "Type command here..." {
			t.Errorf("Expected wizard reset on out-of-range shortcut, got activeWiz=%v placeholder=%q", tm.activeWiz, tm.textInput.Placeholder)
		}
		if !strings.Contains(tm.commandReadout, "1 and 2") {
			t.Errorf("Expected valid range (1 and 2) in error, got %q", tm.commandReadout)
		}
	})

	t.Run("invalid_guided_selections_reprompt_and_retry", func(t *testing.T) {
		mockClient := &mockBeerKellerClient{
			cellarRes: sampleRemoveCellarResponse(),
		}
		m := initialModel(mockClient, nil)
		m, cmd := typeCommand(m, "remove")
		m, _ = m.Update(mustRunCmd(t, cmd))

		for _, badInput := range []string{"abc", "0", "-1", "99999999999999999999999", "5"} {
			m, cmd = typeCommand(m, badInput)
			if cmd != nil {
				t.Errorf("Expected nil cmd on invalid guided input %q", badInput)
			}
			tm := m.(tuiModel)
			if tm.activeWiz == wizardNone {
				t.Errorf("Expected wizard to stay active after invalid guided input %q", badInput)
			}
			if !strings.Contains(tm.commandReadout, "1 and 2") {
				t.Errorf("Expected range error (1 and 2) for %q, got:\n%s", badInput, tm.commandReadout)
			}
			if !strings.Contains(tm.commandReadout, "1. Brewery A - Beer 1 (2.00 units)") || !strings.Contains(tm.commandReadout, "2. Brewery B - Beer 2 (4.00 units)") {
				t.Errorf("Expected numbered list to remain visible after %q, got:\n%s", badInput, tm.commandReadout)
			}
		}

		// User can now pick valid item #1
		m, _ = typeCommand(m, "1")
		if !strings.Contains(m.(tuiModel).commandReadout, `Remove #1 "Brewery A - Beer 1 (2.00 units)"? (y/n)`) {
			t.Errorf("Expected retry with '1' to transition to confirm, got:\n%s", m.(tuiModel).commandReadout)
		}
	})
}

func TestRemoveWizardUncachedMetadata(t *testing.T) {
	cases := []struct {
		name     string
		beer     *pb.Beer
		entry    *pb.CellarEntry
		wantID   string
		wantText []string
	}{
		{
			name:     "empty_brewery_and_name",
			beer:     &pb.Beer{Id: 98765, Units: 1.75},
			entry:    &pb.CellarEntry{BeerId: 98765, DateAdded: 1700000300, SizeFlOz: 12},
			wantID:   "[ID: 98765]",
			wantText: []string{"[ID: 98765]", "1.75 units"},
		},
		{
			name:     "empty_name_only",
			beer:     &pb.Beer{Id: 54321, Brewery: "Mystery Brewery", Units: 2.25},
			entry:    &pb.CellarEntry{BeerId: 54321, DateAdded: 1700000400, SizeFlOz: 16},
			wantID:   "[ID: 54321]",
			wantText: []string{"Mystery Brewery", "[ID: 54321]", "2.25 units"},
		},
		{
			name:     "empty_brewery_only",
			beer:     &pb.Beer{Id: 11111, Name: "Mystery IPA", Units: 3.0},
			entry:    &pb.CellarEntry{BeerId: 11111, DateAdded: 1700000500, SizeFlOz: 12},
			wantID:   "[ID: 11111]",
			wantText: []string{"Mystery IPA", "[ID: 11111]", "3.00 units"},
		},
		{
			name:     "nil_beer_entry",
			beer:     nil,
			entry:    &pb.CellarEntry{BeerId: 22222, DateAdded: 1700000600, SizeFlOz: 12},
			wantID:   "[ID: 0]",
			wantText: []string{"[ID: 0]", "0.00 units"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var removedReq *pb.RemoveBeerRequest
			mockClient := &mockBeerKellerClient{
				cellarRes: &pb.GetCellarResponse{
					Beers:   []*pb.Beer{tc.beer},
					Entries: []*pb.CellarEntry{tc.entry},
				},
				removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
					removedReq = in
					return &pb.RemoveBeerResponse{}, nil
				},
			}

			m := initialModel(mockClient, nil)
			m, cmd := typeCommand(m, "remove")
			m, _ = m.Update(mustRunCmd(t, cmd))

			tm := m.(tuiModel)
			for _, w := range tc.wantText {
				if !strings.Contains(tm.commandReadout, w) {
					t.Errorf("Expected readout to contain %q, got:\n%s", w, tm.commandReadout)
				}
			}

			m, _ = typeCommand(m, "1")
			if !strings.Contains(m.(tuiModel).commandReadout, tc.wantID) {
				t.Errorf("Expected confirm prompt to include %q, got:\n%s", tc.wantID, m.(tuiModel).commandReadout)
			}

			m, cmd = typeCommand(m, "y")
			m, _ = m.Update(mustRunCmd(t, cmd))
			if removedReq == nil || removedReq.GetBeerId() != tc.entry.GetBeerId() {
				t.Errorf("Expected RemoveBeerRequest with BeerId %d, got %+v", tc.entry.GetBeerId(), removedReq)
			}
			if !strings.Contains(m.(tuiModel).commandReadout, tc.wantID) || !strings.Contains(m.(tuiModel).commandReadout, "Removed ") {
				t.Errorf("Expected removal success readout with fallback ID %q, got %q", tc.wantID, m.(tuiModel).commandReadout)
			}
		})
	}
}

func TestRemoveWizardBusyGuard(t *testing.T) {
	getCellarCalls := 0
	removeCalls := 0
	mockClient := &mockBeerKellerClient{
		getCellarFunc: func(ctx context.Context, in *pb.GetCellarRequest) (*pb.GetCellarResponse, error) {
			getCellarCalls++
			return sampleRemoveCellarResponse(), nil
		},
		removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
			removeCalls++
			return &pb.RemoveBeerResponse{}, nil
		},
	}

	m := initialModel(mockClient, nil)
	m, fetchCmd := typeCommand(m, "remove 1")

	// Busy guard during removeStepFetching: pressing Enter should do nothing
	m, busyCmd1 := typeCommand(m, "1")
	if busyCmd1 != nil {
		t.Errorf("Expected nil cmd while fetching, got %v", busyCmd1)
	}
	if !strings.Contains(m.(tuiModel).commandReadout, "Fetching cellar...") {
		t.Errorf("Expected readout to stay 'Fetching cellar...', got %q", m.(tuiModel).commandReadout)
	}

	// Complete fetch
	m, _ = m.Update(mustRunCmd(t, fetchCmd))
	if getCellarCalls != 1 {
		t.Errorf("Expected 1 GetCellar call, got %d", getCellarCalls)
	}

	// Submit 'y' to enter removeStepRemoving
	m, removeCmd := typeCommand(m, "y")
	if removeCmd == nil {
		t.Fatal("Expected removeCmd, got nil")
	}

	// Busy guard during removeStepRemoving: pressing Enter again should do nothing
	m, busyCmd2 := typeCommand(m, "y")
	if busyCmd2 != nil {
		t.Errorf("Expected nil cmd while removing, got %v", busyCmd2)
	}

	// Complete original removal
	m, _ = m.Update(removeCmd())
	if removeCalls != 1 {
		t.Errorf("Expected 1 RemoveBeer call, got %d", removeCalls)
	}
}

func TestRemoveWizardErrorStates(t *testing.T) {
	t.Run("get_cellar_error", func(t *testing.T) {
		mockClient := &mockBeerKellerClient{
			cellarErr: status.Error(codes.Unauthenticated, "unauthenticated"),
		}
		m := initialModel(mockClient, nil)
		m, cmd := typeCommand(m, "remove")
		m, _ = m.Update(mustRunCmd(t, cmd))

		tm := m.(tuiModel)
		if tm.activeWiz != wizardNone || tm.textInput.Placeholder != "Type command here..." {
			t.Errorf("Expected wizard reset on GetCellar error, got activeWiz=%v placeholder=%q", tm.activeWiz, tm.textInput.Placeholder)
		}
		if !strings.Contains(tm.commandReadout, "unauthenticated") {
			t.Errorf("Expected error details in readout, got %q", tm.commandReadout)
		}
	})

	for _, staleCode := range []codes.Code{codes.FailedPrecondition, codes.OutOfRange} {
		t.Run("stale_cellar_"+staleCode.String(), func(t *testing.T) {
			getCellarCalls := 0
			mockClient := &mockBeerKellerClient{
				getCellarFunc: func(ctx context.Context, in *pb.GetCellarRequest) (*pb.GetCellarResponse, error) {
					getCellarCalls++
					return sampleRemoveCellarResponse(), nil
				},
				removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
					return nil, status.Error(staleCode, "cellar changed")
				},
			}

			m := initialModel(mockClient, nil)
			m, cmd := typeCommand(m, "remove 1")
			m, _ = m.Update(mustRunCmd(t, cmd))
			m, cmd = typeCommand(m, "y")
			m, summaryCmd := m.Update(mustRunCmd(t, cmd))

			tm := m.(tuiModel)
			if tm.activeWiz != wizardNone || tm.textInput.Placeholder != "Type command here..." {
				t.Errorf("Expected wizard reset, got activeWiz=%v placeholder=%q", tm.activeWiz, tm.textInput.Placeholder)
			}
			want := "Cellar changed before removal completed—no beer was removed. Please try again."
			if !strings.Contains(tm.commandReadout, want) {
				t.Errorf("Expected %q, got %q", want, tm.commandReadout)
			}
			if summaryCmd == nil {
				t.Fatal("Expected summary refresh cmd on stale cellar error, got nil")
			}
			_, _ = m.Update(summaryCmd())
			if getCellarCalls != 2 {
				t.Errorf("Expected summary refresh to call GetCellar (total 2 calls), got %d", getCellarCalls)
			}
		})
	}

	for _, errCode := range []codes.Code{codes.PermissionDenied, codes.Unavailable, codes.DeadlineExceeded} {
		t.Run("remove_error_"+errCode.String(), func(t *testing.T) {
			mockClient := &mockBeerKellerClient{
				cellarRes: sampleRemoveCellarResponse(),
				removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
					return nil, status.Error(errCode, "backend error")
				},
			}

			m := initialModel(mockClient, nil)
			m, cmd := typeCommand(m, "remove 1")
			m, _ = m.Update(mustRunCmd(t, cmd))
			m, cmd = typeCommand(m, "y")
			m, nextCmd := m.Update(mustRunCmd(t, cmd))

			tm := m.(tuiModel)
			if tm.activeWiz != wizardNone || tm.textInput.Placeholder != "Type command here..." {
				t.Errorf("Expected wizard reset, got activeWiz=%v placeholder=%q", tm.activeWiz, tm.textInput.Placeholder)
			}
			if !strings.Contains(tm.commandReadout, "Error removing beer:") {
				t.Errorf("Expected 'Error removing beer:' in readout, got %q", tm.commandReadout)
			}
			if nextCmd != nil {
				t.Errorf("Expected nil cmd on non-stale RemoveBeer error, got %v", nextCmd)
			}
		})
	}

	t.Run("summary_refresh_error_preserves_removal_success", func(t *testing.T) {
		cellarCalls := 0
		mockClient := &mockBeerKellerClient{
			getCellarFunc: func(ctx context.Context, in *pb.GetCellarRequest) (*pb.GetCellarResponse, error) {
				cellarCalls++
				if cellarCalls == 1 {
					return sampleRemoveCellarResponse(), nil
				}
				return nil, status.Error(codes.Unavailable, "summary backend unavailable")
			},
			removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
				return &pb.RemoveBeerResponse{}, nil
			},
		}

		m := initialModel(mockClient, nil)
		m, cmd := typeCommand(m, "remove 1")
		m, _ = m.Update(mustRunCmd(t, cmd))
		m, cmd = typeCommand(m, "y")
		m, summaryCmd := m.Update(mustRunCmd(t, cmd))
		if summaryCmd == nil {
			t.Fatal("Expected summary refresh cmd, got nil")
		}

		// Summary refresh fails -> summary pane updates, removal success readout preserved
		m, _ = m.Update(summaryCmd())
		rendered := m.View()
		if !strings.Contains(rendered, `Removed "Brewery A - Beer 1 (2.00 units)" from cellar.`) {
			t.Errorf("Expected removal success message preserved in View, got:\n%s", rendered)
		}
		if !strings.Contains(rendered, "Error loading summary:") {
			t.Errorf("Expected summary error in summary pane, got:\n%s", rendered)
		}
	})
}

type drainStepMsg struct {
	label string
}

type drainTestModel struct {
	steps      []string
	onStepFunc func(label string, count int) tea.Cmd
}

func (m drainTestModel) Init() tea.Cmd {
	return nil
}

func (m drainTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case drainStepMsg:
		m.steps = append(m.steps, msg.label)
		if m.onStepFunc != nil {
			return m, m.onStepFunc(msg.label, len(m.steps))
		}
	}
	return m, nil
}

func (m drainTestModel) View() string {
	return strings.Join(m.steps, ",")
}

func TestDrainTuiCmds(t *testing.T) {
	t.Run("nil_and_nonpositive_budget", func(t *testing.T) {
		m := drainTestModel{}
		res := drainTuiCmds(m, nil, 10).(drainTestModel)
		if len(res.steps) != 0 {
			t.Errorf("Expected 0 steps for nil cmd, got %v", res.steps)
		}

		called := false
		cmd := func() tea.Msg {
			called = true
			return drainStepMsg{label: "should-not-run"}
		}
		res = drainTuiCmds(m, cmd, 0).(drainTestModel)
		if called || len(res.steps) != 0 {
			t.Errorf("Expected cmd not to run when maxSteps=0, called=%v steps=%v", called, res.steps)
		}
		res = drainTuiCmds(m, cmd, -1).(drainTestModel)
		if called || len(res.steps) != 0 {
			t.Errorf("Expected cmd not to run when maxSteps=-1, called=%v steps=%v", called, res.steps)
		}
	})

	t.Run("chained_cmds", func(t *testing.T) {
		var cmd3 tea.Cmd = func() tea.Msg { return drainStepMsg{label: "step3"} }
		var cmd2 tea.Cmd = func() tea.Msg { return drainStepMsg{label: "step2"} }
		var cmd1 tea.Cmd = func() tea.Msg { return drainStepMsg{label: "step1"} }

		m := drainTestModel{
			onStepFunc: func(label string, _ int) tea.Cmd {
				switch label {
				case "step1":
					return cmd2
				case "step2":
					return cmd3
				default:
					return nil
				}
			},
		}

		res := drainTuiCmds(m, cmd1, maxTuiTestDrainSteps).(drainTestModel)
		want := []string{"step1", "step2", "step3"}
		if strings.Join(res.steps, ",") != strings.Join(want, ",") {
			t.Errorf("Expected steps %v, got %v", want, res.steps)
		}
	})

	t.Run("batch_msg_and_nested_batch", func(t *testing.T) {
		cmdA := func() tea.Msg { return drainStepMsg{label: "A"} }
		cmdB := func() tea.Msg { return nil }
		cmdC := func() tea.Msg { return drainStepMsg{label: "C"} }
		cmdD := func() tea.Msg { return drainStepMsg{label: "D"} }

		m := drainTestModel{
			onStepFunc: func(label string, _ int) tea.Cmd {
				if label == "A" {
					return cmdD
				}
				return nil
			},
		}

		initial := tea.Batch(cmdA, nil, tea.Batch(cmdB, cmdC))
		res := drainTuiCmds(m, initial, maxTuiTestDrainSteps).(drainTestModel)
		got := strings.Join(res.steps, ",")
		for _, want := range []string{"A", "C", "D"} {
			if !strings.Contains(got, want) {
				t.Errorf("Expected step %q in %v", want, res.steps)
			}
		}
		if len(res.steps) != 3 {
			t.Errorf("Expected 3 steps (A, C, D), got %d: %v", len(res.steps), res.steps)
		}
	})

	t.Run("bounded_step_guard", func(t *testing.T) {
		var loopCmd func() tea.Msg
		loopCmd = func() tea.Msg {
			return drainStepMsg{label: "tick"}
		}

		m := drainTestModel{
			onStepFunc: func(_ string, _ int) tea.Cmd {
				return loopCmd
			},
		}

		res := drainTuiCmds(m, loopCmd, 5).(drainTestModel)
		if len(res.steps) != 5 {
			t.Errorf("Expected bounded guard to stop at 5 steps, got %d: %v", len(res.steps), res.steps)
		}
	})

	t.Run("quit_msg_stops_draining", func(t *testing.T) {
		followUpRan := false
		followUpCmd := func() tea.Msg {
			followUpRan = true
			return drainStepMsg{label: "after-quit"}
		}

		m := drainTestModel{}
		res := drainTuiCmds(m, tea.Batch(tea.Quit, followUpCmd), maxTuiTestDrainSteps).(drainTestModel)
		if followUpRan || len(res.steps) != 0 {
			t.Errorf("Expected QuitMsg to halt draining immediately, followUpRan=%v steps=%v", followUpRan, res.steps)
		}
	})

	t.Run("tui_remove_chains_summary_refresh", func(t *testing.T) {
		cellarCalls := 0
		mockClient := &mockBeerKellerClient{
			getCellarFunc: func(ctx context.Context, in *pb.GetCellarRequest) (*pb.GetCellarResponse, error) {
				cellarCalls++
				if cellarCalls == 1 {
					return sampleRemoveCellarResponse(), nil
				}
				return &pb.GetCellarResponse{
					Username: "testuser",
					State:    pb.User_STATE_AUTHORIZED,
					Beers: []*pb.Beer{
						{Id: 202, Name: "Beer 2", Brewery: "Brewery B", Units: 4.0},
					},
					Entries: []*pb.CellarEntry{
						{BeerId: 202, DateAdded: 1700000200, SizeFlOz: 16},
					},
				}, nil
			},
			removeBeerFunc: func(ctx context.Context, in *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error) {
				return &pb.RemoveBeerResponse{}, nil
			},
			weekdayRes: &pb.GetBeerResponse{},
			weekendRes: &pb.GetBeerResponse{
				Beers: []*pb.Beer{{Id: 202, Name: "Beer 2", Brewery: "Brewery B", Units: 4.0}},
			},
		}

		m := initialModel(mockClient, nil)
		var cmd tea.Cmd
		m, cmd = typeCommand(m, "remove 1")
		m = drainTuiCmds(m, cmd, maxTuiTestDrainSteps)

		m, cmd = typeCommand(m, "y")
		m = drainTuiCmds(m, cmd, maxTuiTestDrainSteps)

		rendered := m.View()
		if !strings.Contains(rendered, `Removed "Brewery A - Beer 1 (2.00 units)" from cellar.`) {
			t.Errorf("Expected removal readout in View, got:\n%s", rendered)
		}
		if !strings.Contains(rendered, "Cellar Size & Split: 1 Beers") {
			t.Errorf("Expected refreshed summary count 'Cellar Size & Split: 1 Beers' in View, got:\n%s", rendered)
		}
	})
}

func TestAutoUpdateTUITriggersUpdateOnNewerTag(t *testing.T) {
	origVersion := Version
	origURL := githubTagsURL
	origInstall := goInstallRunner
	origExec := execRunner
	defer func() {
		Version = origVersion
		githubTagsURL = origURL
		goInstallRunner = origInstall
		execRunner = origExec
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"name": "v0.119.0"}]`))
	}))
	defer server.Close()

	var installedPkg string
	var restarted bool
	Version = "v0.115.0"
	githubTagsURL = server.URL
	goInstallRunner = func(pkg string) error {
		installedPkg = pkg
		return nil
	}
	execRunner = func(argv0 string, argv []string, envv []string) error {
		restarted = true
		return nil
	}

	autoUpdateTUI()

	expectedPkg := "github.com/brotherlogic/beerkellar/beerkellar_cli@v0.119.0"
	if installedPkg != expectedPkg {
		t.Errorf("Expected go install package %q, got %q", expectedPkg, installedPkg)
	}
	if !restarted {
		t.Error("Expected execRunner to be called to restart the CLI, but it was not")
	}
}

func TestAutoUpdateTUISkipsWhenUpToDate(t *testing.T) {
	origVersion := Version
	origURL := githubTagsURL
	origInstall := goInstallRunner
	origExec := execRunner
	defer func() {
		Version = origVersion
		githubTagsURL = origURL
		goInstallRunner = origInstall
		execRunner = origExec
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"name": "v0.119.0"}]`))
	}))
	defer server.Close()

	installCalled := false
	execCalled := false
	Version = "v0.119.0"
	githubTagsURL = server.URL
	goInstallRunner = func(pkg string) error {
		installCalled = true
		return nil
	}
	execRunner = func(argv0 string, argv []string, envv []string) error {
		execCalled = true
		return nil
	}

	autoUpdateTUI()

	if installCalled {
		t.Error("Expected goInstallRunner not to be called when already up to date")
	}
	if execCalled {
		t.Error("Expected execRunner not to be called when already up to date")
	}
}

func TestAutoUpdateTUIHandlesNon200Status(t *testing.T) {
	origVersion := Version
	origURL := githubTagsURL
	origInstall := goInstallRunner
	origExec := execRunner
	defer func() {
		Version = origVersion
		githubTagsURL = origURL
		goInstallRunner = origInstall
		execRunner = origExec
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server.Close()

	installCalled := false
	execCalled := false
	Version = "v0.115.0"
	githubTagsURL = server.URL
	goInstallRunner = func(pkg string) error {
		installCalled = true
		return nil
	}
	execRunner = func(argv0 string, argv []string, envv []string) error {
		execCalled = true
		return nil
	}

	autoUpdateTUI()

	if installCalled {
		t.Error("Expected goInstallRunner not to be called on non-200 status")
	}
	if execCalled {
		t.Error("Expected execRunner not to be called on non-200 status")
	}
}
