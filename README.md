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
3. Add a beer (requires ID, quantity, and size in fl oz):
   ```bash
   beerkellar_cli add --id <beer_id> --quantity 1 --size 12
   ```
4. View your cellar and a summary of beer types:
   ```bash
   beerkellar_cli cellar
   ```
   Shows all beers in your cellar along with a summary of the number of weekday (< 2.5 units) and non-weekday beers.
5. Pull a beer to drink:
   ```bash
   beerkellar_cli pull [--weekday]
   ```
   The `pull` command chooses a beer you haven't drunk recently (or at all). Use `--weekday` to limit choices to beers with 2.5 units of alcohol or less.
6. View your drunk history:
   ```bash
   beerkellar_cli drunk [--count 10]
   ```
   Shows your recently consumed beers, including date and alcohol units.

## Documentation

## User States

Beerkellar tracks user authentication through three primary states:
- **LOGGING_IN**: User has initiated the login process.
- **LOGGED_IN**: User has authenticated with Untappd and we have received an access token.
- **AUTHORIZED**: The system has successfully retrieved the user's profile information from Untappd (username and user ID).

All API methods (except login-related ones) require the user to be in the **AUTHORIZED** state.

## Default Initialization

If a user gets placed into the **AUTHORIZED** state and attempts to access an empty cellar or view their drunk history, the system will now automatically provision and store underlying empty objects upon first query. This prevents downstream apps or the client interface from running into confusing `NotFound` scenarios.

## Caching and Offline Updates

When managing beers interacting with the Untappd API, all calls are securely pushed to a background process queue to avoid quota limits. To ensure robust fetching, failed metadata calls will be transparently retried the next time a user opens their cellar view.

Background tasks, such as automated user refreshes, now include a one-minute initial delay upon server startup to ensure all systems are fully initialized before processing begins.

## Development

### Dev Container
This project includes a VS Code Dev Container configuration for a consistent development environment. It is configured to automatically set the working directory to `/workspaces/beerkellar`. Detailed project guidance for AI assistants is available in `GEMINI.md`.

### Prerequisites
- Go 1.22+
- Docker (for integration tests)

### Commands
- Build: `go build ./...`
- Test: `go test ./...`
