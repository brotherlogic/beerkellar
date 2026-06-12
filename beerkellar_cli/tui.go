package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/browser"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/brotherlogic/beerkellar/proto"
)

type wizardType int

const (
	wizardNone wizardType = iota
	wizardAdd
	wizardDrink
)

const defaultTimeout = 5 * time.Second

type addWizardState struct {
	step     int // 0: ID, 1: Quantity, 2: Size
	beerID   int64
	quantity int32
	size     int32
}

type drinkWizardState struct {
	beerID int64
}

// tuiModel implements the tea.Model interface for the Bubble Tea program.
type tuiModel struct {
	client         pb.BeerKellerClient
	googleClient   pb.BeerKellerGoogleClient
	cellarSummary  string
	commandReadout string
	untappdStatus  string
	googleStatus   string
	err            error

	// Input and wizard states
	textInput   textinput.Model
	activeWiz   wizardType
	addWiz      addWizardState
	drinkWiz    drinkWizardState

	// Terminal size
	width  int
	height int
}

func initialModel(client pb.BeerKellerClient, googleClient pb.BeerKellerGoogleClient) tea.Model {
	ti := textinput.New()
	ti.Placeholder = "Type command here..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	return tuiModel{
		client:         client,
		googleClient:   googleClient,
		cellarSummary:  "CELLAR SUMMARY\nCellar Size & Split: 0 Beers (0 Weekday, 0 Weekend)\nNext Weekday Candidate: None\nNext Weekend Candidate: None",
		commandReadout: "",
		untappdStatus:  "Untappd: Disconnected",
		googleStatus:   "Google Tasks: Disconnected",
		textInput:      ti,
		activeWiz:      wizardNone,
	}
}

type cellarSummaryMsg struct {
	cellar  *pb.GetCellarResponse
	weekday *pb.GetBeerResponse
	weekend *pb.GetBeerResponse
	err     error
}

type tickMsg time.Time

func (m tuiModel) fetchCellarSummary() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return cellarSummaryMsg{}
		}
		// Set a conservative 10-second timeout for retrieving cellar stats and recommendations.
		ctx, cancel := m.getContext(time.Second * 10)
		defer cancel()

		cellar, err := m.client.GetCellar(ctx, &pb.GetCellarRequest{})
		if err != nil {
			return cellarSummaryMsg{err: err}
		}

		weekdayReq := &pb.GetBeerRequest{
			NoRepeat: true,
			Requirements: []*pb.BeerRequirement{
				{
					Strategy: pb.BeerRequirement_STRATEGY_LEAST_RECENTLY_DRUNK,
					MaxUnits: 3,
				},
			},
		}
		weekdayRes, _ := m.client.GetBeer(ctx, weekdayReq)

		weekendReq := &pb.GetBeerRequest{
			NoRepeat: true,
			Requirements: []*pb.BeerRequirement{
				{
					Strategy: pb.BeerRequirement_STRATEGY_LEAST_RECENTLY_DRUNK,
				},
			},
		}
		weekendRes, _ := m.client.GetBeer(ctx, weekendReq)

		return cellarSummaryMsg{
			cellar:  cellar,
			weekday: weekdayRes,
			weekend: weekendRes,
		}
	}
}

func (m *tuiModel) updateCellarSummary(msg cellarSummaryMsg) {
	if msg.err != nil {
		m.cellarSummary = fmt.Sprintf("CELLAR SUMMARY\nError loading summary: %v", msg.err)
		return
	}

	if msg.cellar == nil {
		return
	}

	var total, weekday, weekend int
	for _, beer := range msg.cellar.GetBeers() {
		total++
		if beer.GetUnits() < 2.5 {
			weekday++
		} else {
			weekend++
		}
	}

	if msg.cellar.GetState() == pb.User_STATE_LOGGED_IN {
		m.untappdStatus = "Untappd: Logged In"
	} else {
		m.untappdStatus = "Untappd: Disconnected"
	}

	weekdayCandidate := "None"
	if msg.weekday != nil && len(msg.weekday.GetBeers()) > 0 {
		beer := msg.weekday.GetBeers()[0]
		weekdayCandidate = fmt.Sprintf("%s - %s (%.2f units)", beer.GetBrewery(), beer.GetName(), beer.GetUnits())
	}

	weekendCandidate := "None"
	if msg.weekend != nil && len(msg.weekend.GetBeers()) > 0 {
		beer := msg.weekend.GetBeers()[0]
		weekendCandidate = fmt.Sprintf("%s - %s (%.2f units)", beer.GetBrewery(), beer.GetName(), beer.GetUnits())
	}

	m.cellarSummary = fmt.Sprintf("CELLAR SUMMARY\nCellar Size & Split: %d Beers (%d Weekday, %d Weekend)\nNext Weekday Candidate: %s\nNext Weekend Candidate: %s",
		total, weekday, weekend, weekdayCandidate, weekendCandidate)
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.fetchCellarSummary(),
		m.checkInitialStatus(),
		tea.Tick(time.Hour, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

type authStatusMsg struct {
	untappdStatus string
	googleStatus  string
}

type loginInitiatedMsg struct {
	code string
	err  error
}

type loginPollMsg struct {
	code    string
	attempt int
	token   *pb.GetAuthTokenResponse
	err     error
}

type googleLoginInitiatedMsg struct {
	err error
}

type googlePollMsg struct {
	attempt int
	err     error
}

type cmdResultMsg struct {
	content string
	err     error
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			input := strings.TrimSpace(m.textInput.Value())
			m.textInput.SetValue("")

			if m.activeWiz == wizardNone {
				return m.handleRootCommand(input)
			} else {
				return m.handleWizardInput(input)
			}
		}

	case authStatusMsg:
		m.untappdStatus = msg.untappdStatus
		m.googleStatus = msg.googleStatus
		return m, nil

	case loginInitiatedMsg:
		if msg.err != nil {
			m.commandReadout = fmt.Sprintf("COMMAND READOUT\nError starting login: %v", msg.err)
			return m, nil
		}
		m.commandReadout = "COMMAND READOUT\nOpening browser... Please log in on Untappd."
		return m, m.pollLogin(msg.code, 1)

	case loginPollMsg:
		if msg.err == nil && msg.token != nil && msg.token.GetCode() != "" {
			err := saveToken(msg.token)
			if err != nil {
				m.commandReadout = fmt.Sprintf("COMMAND READOUT\nLogin succeeded but failed to save token: %v", err)
				return m, nil
			}
			m.untappdStatus = "Untappd: Logged In"
			m.commandReadout = "COMMAND READOUT\nSuccessfully logged in to Untappd!"
			return m, tea.Batch(m.checkInitialStatus(), m.runGetCellar())
		}
		if msg.attempt < 12 {
			return m, m.pollLogin(msg.code, msg.attempt+1)
		}
		m.commandReadout = "COMMAND READOUT\nLogin timed out or failed."
		return m, nil

	case googleLoginInitiatedMsg:
		if msg.err != nil {
			m.commandReadout = fmt.Sprintf("COMMAND READOUT\nError starting Google login: %v", msg.err)
			return m, nil
		}
		m.commandReadout = "COMMAND READOUT\nOpening browser... Please log in to Google."
		return m, m.pollGoogle(1)

	case googlePollMsg:
		if msg.err == nil {
			m.googleStatus = "Google Tasks: Linked"
			m.commandReadout = "COMMAND READOUT\nSuccessfully linked Google Account!"
			return m, nil
		}
		if msg.attempt < 12 {
			return m, m.pollGoogle(msg.attempt+1)
		}
		m.commandReadout = "COMMAND READOUT\nGoogle login timed out or failed."
		return m, nil

	case cmdResultMsg:
		if msg.err != nil {
			m.commandReadout = fmt.Sprintf("COMMAND READOUT\nError: %v", msg.err)
		} else {
			m.commandReadout = fmt.Sprintf("COMMAND READOUT\n%s", msg.content)
		}
		return m, nil

	case cellarSummaryMsg:
		m.updateCellarSummary(msg)
		return m, nil

	case tickMsg:
		return m, tea.Batch(
			m.fetchCellarSummary(),
			tea.Tick(time.Hour, func(t time.Time) tea.Msg {
				return tickMsg(t)
			}),
		)
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m tuiModel) handleRootCommand(input string) (tuiModel, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}

	cmdName := strings.ToLower(parts[0])
	switch cmdName {
	case "exit", "quit":
		return m, tea.Quit

	case "add":
		m.activeWiz = wizardAdd
		m.addWiz = addWizardState{step: 0}
		m.textInput.Placeholder = "Enter Beer ID"
		return m, nil

	case "drink":
		m.activeWiz = wizardDrink
		m.textInput.Placeholder = "Enter Beer ID to drink"
		return m, nil

	case "cellar":
		return m, m.runGetCellar()

	case "pull":
		return m, m.runPullBeer()

	case "drunk":
		return m, m.runGetDrunk()

	case "login":
		return m, m.runLogin()

	case "google":
		if len(parts) >= 2 {
			sub := strings.ToLower(parts[1])
			if sub == "login" {
				return m, m.runGoogleLogin()
			} else if sub == "tasks" && len(parts) >= 3 {
				enable := strings.ToLower(parts[2]) == "on"
				return m, m.runToggleGoogleTasks(enable)
			}
		}
		m.commandReadout = "COMMAND READOUT\nInvalid google command. Try 'google login' or 'google tasks [on|off]'"
		return m, nil

	default:
		m.commandReadout = fmt.Sprintf("COMMAND READOUT\nUnknown command: %s", input)
		return m, nil
	}
}

func (m tuiModel) handleWizardInput(input string) (tuiModel, tea.Cmd) {
	switch m.activeWiz {
	case wizardAdd:
		switch m.addWiz.step {
		case 0:
			id, err := strconv.ParseInt(input, 10, 64)
			if err != nil {
				m.commandReadout = "COMMAND READOUT\nInvalid ID. Please enter a number."
				return m, nil
			}
			m.addWiz.beerID = id
			m.addWiz.step = 1
			m.textInput.Placeholder = "Enter Quantity"
			return m, nil

		case 1:
			qty, err := strconv.ParseInt(input, 10, 32)
			if err != nil {
				m.commandReadout = "COMMAND READOUT\nInvalid Quantity. Please enter a number."
				return m, nil
			}
			m.addWiz.quantity = int32(qty)
			m.addWiz.step = 2
			m.textInput.Placeholder = "Enter Size (fl oz)"
			return m, nil

		case 2:
			sz, err := strconv.ParseInt(input, 10, 32)
			if err != nil {
				m.commandReadout = "COMMAND READOUT\nInvalid Size. Please enter a number."
				return m, nil
			}
			m.addWiz.size = int32(sz)
			m.activeWiz = wizardNone
			m.textInput.Placeholder = "Type command here..."
			return m, m.runAddBeer(m.addWiz.beerID, m.addWiz.quantity, m.addWiz.size)
		}

	case wizardDrink:
		id, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			m.commandReadout = "COMMAND READOUT\nInvalid ID. Please enter a number."
			return m, nil
		}
		m.drinkWiz.beerID = id
		m.activeWiz = wizardNone
		m.textInput.Placeholder = "Type command here..."
		return m, m.runDrinkBeer(m.drinkWiz.beerID)
	}

	return m, nil
}

// gRPC commands executed asynchronously

func (m tuiModel) getContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		return ctx, cancel
	}

	text, err := os.ReadFile(fmt.Sprintf("%v/.beerkellar", dirname))
	if err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		return ctx, cancel
	}

	user := &pb.GetAuthTokenResponse{}
	err = prototext.Unmarshal(text, user)
	if err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		return ctx, cancel
	}

	mContext := metadata.AppendToOutgoingContext(context.Background(), "auth-token", user.GetCode())
	ctx, cancel := context.WithTimeout(mContext, timeout)
	return ctx, cancel
}

func saveToken(auth *pb.GetAuthTokenResponse) error {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("unable to get home dir: %w", err)
	}
	f, err := os.OpenFile(fmt.Sprintf("%v/.beerkellar", dirname), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to open file: %w", err)
	}
	defer f.Close()
	if err := proto.MarshalText(f, auth); err != nil {
		return fmt.Errorf("unable to save token: %w", err)
	}
	return nil
}

func (m tuiModel) checkInitialStatus() tea.Cmd {
	return func() tea.Msg {
		untappd := "Untappd: Disconnected"
		google := "Google Tasks: Disconnected"

		ctx, cancel := m.getContext(defaultTimeout)
		defer cancel()

		if m.client != nil {
			cellar, err := m.client.GetCellar(ctx, &pb.GetCellarRequest{})
			if err == nil {
				untappd = "Untappd: Logged In"
				if cellar.GetState() == pb.User_STATE_AUTHORIZED {
					untappd = "Untappd: Logged In"
				}
				if m.googleClient != nil {
					_, err := m.googleClient.ToggleGoogleTasks(ctx, &pb.ToggleGoogleTasksRequest{Enable: true})
					if err == nil {
						google = "Google Tasks: Linked"
					}
				}
			}
		}
		return authStatusMsg{
			untappdStatus: untappd,
			googleStatus:  google,
		}
	}
}

func (m tuiModel) runGetCellar() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return cmdResultMsg{content: "Not running command in mock (cellar)"}
		}
		ctx, cancel := m.getContext(defaultTimeout)
		defer cancel()
		cellar, err := m.client.GetCellar(ctx, &pb.GetCellarRequest{})
		if err != nil {
			return cmdResultMsg{err: err}
		}
		var sb strings.Builder
		// Omit the state readout if the user is in the default, fully authorized state (STATE_AUTHORIZED)
		// to avoid cluttering the cellar output. Other transition states are still displayed.
		if cellar.GetState() == pb.User_STATE_AUTHORIZED {
			sb.WriteString(fmt.Sprintf("User: %v\n", cellar.GetUsername()))
		} else {
			sb.WriteString(fmt.Sprintf("User: %v (State: %v)\n", cellar.GetUsername(), cellar.GetState()))
		}
		for i, beer := range cellar.GetBeers() {
			sb.WriteString(fmt.Sprintf("%v. %v - %v (%.2f units)\n", i+1, beer.GetBrewery(), beer.GetName(), beer.GetUnits()))
		}
		return cmdResultMsg{content: sb.String()}
	}
}

func (m tuiModel) runPullBeer() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return cmdResultMsg{content: "Not running command in mock (pull)"}
		}
		ctx, cancel := m.getContext(defaultTimeout)
		defer cancel()
		req := &pb.GetBeerRequest{
			NoRepeat: true,
			Requirements: []*pb.BeerRequirement{
				{
					Strategy: pb.BeerRequirement_STRATEGY_LEAST_RECENTLY_DRUNK,
				},
			},
		}
		res, err := m.client.GetBeer(ctx, req)
		if err != nil {
			return cmdResultMsg{err: err}
		}
		if len(res.GetBeers()) > 0 {
			beer := res.GetBeers()[0]
			return cmdResultMsg{content: fmt.Sprintf("Pulled beer: %v - %v (%.2f units)", beer.GetBrewery(), beer.GetName(), beer.GetUnits())}
		}
		return cmdResultMsg{content: "No beers found matching requirements"}
	}
}

func (m tuiModel) runGetDrunk() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return cmdResultMsg{content: "Not running command in mock (drunk)"}
		}
		ctx, cancel := m.getContext(defaultTimeout)
		defer cancel()
		res, err := m.client.GetDrunk(ctx, &pb.GetDrunkRequest{Count: 10})
		if err != nil {
			return cmdResultMsg{err: err}
		}
		var sb strings.Builder
		for _, beer := range res.GetDrunk() {
			dateStr := time.Unix(beer.GetDate(), 0).Format("2006-01-02")
			sb.WriteString(fmt.Sprintf("%v %v - %v (%.2f units)\n", dateStr, beer.GetBrewery(), beer.GetName(), beer.GetUnits()))
		}
		return cmdResultMsg{content: sb.String()}
	}
}

func (m tuiModel) runAddBeer(id int64, qty, sz int32) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return cmdResultMsg{content: fmt.Sprintf("Not running command in mock (add - id:%v qty:%v size:%v)", id, qty, sz)}
		}
		ctx, cancel := m.getContext(defaultTimeout)
		defer cancel()
		res, err := m.client.AddBeer(ctx, &pb.AddBeerRequest{
			BeerId:   id,
			Quantity: qty,
			SizeFlOz: sz,
		})
		if err != nil {
			return cmdResultMsg{err: err}
		}
		return cmdResultMsg{content: fmt.Sprintf("Beers added successfully: %+v", res)}
	}
}

func (m tuiModel) runDrinkBeer(id int64) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return cmdResultMsg{content: fmt.Sprintf("Not running command in mock (drink - id:%v)", id)}
		}
		ctx, cancel := m.getContext(defaultTimeout)
		defer cancel()
		_, err := m.client.DrinkBeer(ctx, &pb.DrinkBeerRequest{BeerId: id})
		if err != nil {
			return cmdResultMsg{err: err}
		}
		return cmdResultMsg{content: fmt.Sprintf("Beer %v recorded as drunk.", id)}
	}
}

func (m tuiModel) runLogin() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return loginInitiatedMsg{err: fmt.Errorf("gRPC client not initialized")}
		}
		ctx, cancel := m.getContext(defaultTimeout)
		defer cancel()
		res, err := m.client.GetLogin(ctx, &pb.GetLoginRequest{})
		if err != nil {
			return loginInitiatedMsg{err: err}
		}
		
		go func() {
			_ = browser.OpenURL(res.GetUrl())
		}()
		
		return loginInitiatedMsg{code: res.GetCode()}
	}
}

func (m tuiModel) pollLogin(code string, attempt int) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(defaultTimeout)
		ctx, cancel := m.getContext(defaultTimeout)
		defer cancel()
		res, err := m.client.GetAuthToken(ctx, &pb.GetAuthTokenRequest{Code: code})
		return loginPollMsg{
			code:    code,
			attempt: attempt,
			token:   res,
			err:     err,
		}
	}
}

func (m tuiModel) runGoogleLogin() tea.Cmd {
	return func() tea.Msg {
		if m.googleClient == nil {
			return googleLoginInitiatedMsg{err: fmt.Errorf("google gRPC client not initialized")}
		}
		ctx, cancel := m.getContext(defaultTimeout)
		defer cancel()
		res, err := m.googleClient.GetGoogleLogin(ctx, &pb.GetGoogleLoginRequest{})
		if err != nil {
			return googleLoginInitiatedMsg{err: err}
		}
		
		go func() {
			_ = browser.OpenURL(res.GetUrl())
		}()
		
		return googleLoginInitiatedMsg{}
	}
}

func (m tuiModel) pollGoogle(attempt int) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(defaultTimeout)
		ctx, cancel := m.getContext(defaultTimeout)
		defer cancel()
		_, err := m.googleClient.ToggleGoogleTasks(ctx, &pb.ToggleGoogleTasksRequest{Enable: true})
		return googlePollMsg{
			attempt: attempt,
			err:     err,
		}
	}
}

func (m tuiModel) runToggleGoogleTasks(enable bool) tea.Cmd {
	return func() tea.Msg {
		if m.googleClient == nil {
			return cmdResultMsg{content: fmt.Sprintf("Not running command in mock (google tasks - enable:%v)", enable)}
		}
		ctx, cancel := m.getContext(defaultTimeout)
		defer cancel()
		_, err := m.googleClient.ToggleGoogleTasks(ctx, &pb.ToggleGoogleTasksRequest{Enable: enable})
		if err != nil {
			return cmdResultMsg{err: err}
		}
		return cmdResultMsg{content: fmt.Sprintf("Google Tasks feature toggled: %v", enable)}
	}
}

const logo = `  ██████╗ ███████╗███████╗██████╗ ██╗  ██╗███████╗██╗     ██╗      █████╗ ██████╗ 
  ██╔══██╗██╔════╝██╔════╝██╔══██╗██║ ██╔╝██╔════╝██║     ██║     ██╔══██╗██╔══██╗
  ██████╔╝█████╗  █████╗  ██████╔╝█████╔╝ █████╗  ██║     ██║     ███████║██████╔╝
  ██╔══██╗██╔══╝  ██╔══╝  ██╔══██╗██╔═██╗ ██╔══╝  ██║     ██║     ██╔══██║██╔══██╗
  ██████╔╝███████╗███████╗██║  ██║██║  ██╗███████╗███████╗███████╗██║  ██║██║  ██║
  ╚══════╝╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝`

func (m tuiModel) View() string {
	docStyle := lipgloss.NewStyle().Padding(1, 2)
	paneStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1)

	logoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFB300")). // Beautiful Amber Gold
		MarginLeft(2).                         // Align with the pane borders
		MarginBottom(1)

	logoView := logoStyle.Render(logo)

	footerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("245"))

	if m.width > 0 {
		w := m.width - 4
		if w > 0 {
			footerStyle = footerStyle.Width(w)
			if w > 2 {
				paneStyle = paneStyle.Width(w - 2)
			}
		}
	}

	summaryView := paneStyle.Render(m.cellarSummary)
	
	// Command Input View
	var inputContent string
	if m.activeWiz != wizardNone {
		inputContent = fmt.Sprintf("COMMAND INPUT (WIZARD MODE)\nPrompt: %s\n%s", m.textInput.Placeholder, m.textInput.View())
	} else {
		inputContent = fmt.Sprintf("COMMAND INPUT\n%s", m.textInput.View())
	}
	inputView := paneStyle.Render(inputContent)

	footerView := footerStyle.Render(fmt.Sprintf(" %s | %s ", m.untappdStatus, m.googleStatus))

	var views []string
	views = append(views, logoView)
	views = append(views, summaryView)
	if m.commandReadout != "" {
		views = append(views, paneStyle.Render(m.commandReadout))
	}
	views = append(views, inputView, footerView)

	return docStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			views...,
		),
	)
}

