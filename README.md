# Beerkellar

Beerkellar is a CLI tool and backend service for managing a personal beer cellar.

## Installation

```bash
go install ./beerkellar_cli
```

## Usage

### Interactive TUI Mode
Run the CLI without any subcommands to enter the interactive TUI dashboard layout:
```bash
beerkellar_cli
```
This launches a three-pane dashboard (Cellar Summary, Command Readout, Command Input) with a status bar. 
Within the TUI:
- The `CELLAR SUMMARY` pane displays total beer counts, weekday/weekend splits, and the next weekday/weekend candidates (with background updates syncing once every hour).
- Type commands directly in the `COMMAND INPUT` pane (e.g. `cellar`, `pull`, `drunk`, `google tasks on`, `exit`, `quit`).
- For multi-step commands like `add` or `drink`, the pane automatically enters a wizard mode, prompting you step-by-step for the required inputs (e.g. Beer ID, Quantity, Size) before performing the action.
- Log in to Untappd using `login` or link your Google account using `google login`. These authentication flows run asynchronously (via `tea.Cmd`) so you can keep interacting with the TUI while the authorization occurs in the background. The status bar will automatically update to reflect when you are successfully logged in or linked.
- Command results are displayed in the middle `COMMAND READOUT` pane.

### 1. Login with Untappd
```bash
beerkellar_cli login
```

### 2. Add Beer
Add a beer to your cellar by ID, specifying quantity and size in fluid ounces.
```bash
beerkellar_cli add --id <beer_id> --quantity 1 --size 12
```

### 3. View Cellar
Show all beers and a summary of weekday vs. non-weekday beers.
```bash
beerkellar_cli cellar
```

### 4. Pull a Beer
Choose a beer you haven't drunk recently. Use `--weekday` to limit choices to beers with 3 units of alcohol or less.
```bash
beerkellar_cli pull [--weekday]
```

### 5. Record a Drink
Mark a beer as consumed.
```bash
beerkellar_cli drink --id <beer_id>
```

### 6. View Drunk History
Show your recently consumed beers.
```bash
beerkellar_cli drunk [--count 10]
```

## Google Tasks Integration

Automatically creates a task when weekday beer (< 3 units) count drops below 4.

### 1. Link Google Account
```bash
beerkellar_cli google login
```

### 2. Enable Tasks
```bash
beerkellar_cli google tasks on
```

## Development

- Build: `go build ./...`
- Test: `go test ./...`
- Integration Tests: Run `go test -v -tags=integration ./integration/...`
- Headless TUI Test Mode: Set the environment variable `TUI_TEST_MODE=true` to run `beerkellar_cli` in a headless line-by-line test loop without initializing a full terminal alternate screen (useful for automated testing/CI environments).
