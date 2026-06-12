# Beerkellar

Beerkellar is a command-line interface (CLI) tool and backend service in Go for managing a personal beer cellar. It integrates with the Untappd API to automatically track consumption and enrich beer metadata, and with Google Tasks to notify you when stocks of weekday beers run low.

## Installation

To build and install the Beerkellar CLI, run:
```bash
go install ./beerkellar_cli
```

---

## Interactive TUI Mode

Run the CLI without any subcommands to enter the interactive Terminal User Interface (TUI) dashboard:
```bash
beerkellar_cli
```

The TUI features a premium three-pane dashboard styled with Lip Gloss, complete with a background updating scheduler and an asynchronous status bar:

### 1. Cellar Summary Pane
Displays real-time stats (cellar size, weekday vs. weekend split) and recommends:
* **Next Weekday Candidate**: The least recently drunk beer under 3 units of alcohol.
* **Next Weekend Candidate**: The overall least recently drunk beer.
* *Updates automatically every hour in the background.*

### 2. Command Readout Pane
Displays logs, command execution results, confirmation prompts, and error outputs.

### 3. Command Input Pane
Type commands directly to interact with your cellar.

#### Standard Commands
* `cellar`: Retrieves and lists your current cellar inventory (hides the user state from the output if it is authorized).
* `pull`: Selects and displays a recommended beer from your cellar.
* `drunk`: Shows the last 10 recently consumed beers.
* `login`: Initiates OAuth authentication with Untappd. Opens your browser automatically, exchanges codes, and updates the status bar once linked.
* `google login`: Links your Google account. Opens your browser automatically, and updates the status bar once linked.
* `google tasks [on|off]`: Enables or disables automatic Google Tasks creation when weekday beers drop below 4.
* `exit` or `quit`: Safely closes the TUI application.

#### Interactive Wizard Commands
* `add`: Prompts you step-by-step for the required inputs:
  1. **Beer ID**
  2. **Quantity**
  3. **Size (fl oz)**
* `drink`: Prompts you for the **Beer ID** to record a beer as consumed.

### 4. Status Bar
Located at the bottom of the screen, showing the live connectivity state:
* `Untappd: Logged In` | `Untappd: Disconnected`
* `Google Tasks: Linked` | `Google Tasks: Disconnected`

---

## Non-Interactive CLI Commands

You can also run commands directly from the shell:

### 1. Authenticate with Untappd
```bash
beerkellar_cli login
```

### 2. Add Beer
```bash
beerkellar_cli add --id <beer_id> --quantity <qty> --size <fl_oz>
```

### 3. View Cellar
```bash
beerkellar_cli cellar
```

### 4. Pull a Beer
Pulls a beer based on least recently drunk. Use the `--weekday` flag to limit selection to beers with 3 units of alcohol or less.
```bash
beerkellar_cli pull [--weekday]
```

### 5. Record a Drink
```bash
beerkellar_cli drink --id <beer_id>
```

### 6. View Drunk History
```bash
beerkellar_cli drunk [--count <n>]
```

### 7. Google Tasks Integration
```bash
beerkellar_cli google login
beerkellar_cli google tasks [on|off]
```

---

## Development & Testing

* **Build**: `go build ./...`
* **Unit Tests**: `go test ./...`
* **Integration Tests**: `go test -v -tags=integration ./integration/...` (Requires Docker runtime)
* **Headless TUI Testing**: Set `TUI_TEST_MODE=true` to run a headless line-by-line TUI loop for integration/CI test environments:
  ```bash
  TUI_TEST_MODE=true beerkellar_cli
  ```
