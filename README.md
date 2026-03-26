# Beerkellar

Beerkellar is a CLI tool and backend service for managing a personal beer cellar.

## Quick Start

1. Install the CLI:
   ```bash
   go install ./beerkellar_cli
   ```
2. Login with Untappd:
   ```bash
   beerkellar_cli login
   ```
3. Add a beer:
   ```bash
   beerkellar_cli add --id <beer_id> --quantity 1
   ```
4. View your cellar:
   ```bash
   beerkellar_cli cellar
   ```

## Documentation

For a detailed overview of the project architecture and features, see [OVERVIEW.md](overview.md).

## Development

### Prerequisites
- Go 1.22+
- Docker (for integration tests)

### Commands
- Build: `go build ./...`
- Test: `go test ./...`
