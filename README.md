# Beerkellar

Beerkellar is a CLI tool and backend service for managing a personal beer cellar.

## Installation

```bash
go install ./beerkellar_cli
```

## Usage

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
Choose a beer you haven't drunk recently. Use `--weekday` to limit choices to beers with 2.5 units of alcohol or less.
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

Automatically creates a task when weekday beer (< 2.5 units) count drops below 4.

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
