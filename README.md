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

## User States

Beerkellar tracks user authentication through three primary states:
- **LOGGING_IN**: User has initiated the login process.
- **LOGGED_IN**: User has authenticated with Untappd and we have received an access token.
- **AUTHORIZED**: The system has successfully retrieved the user's profile information from Untappd (username and user ID).

All API methods (except login-related ones) require the user to be in the **AUTHORIZED** state.

## Default Initialization

If a user gets placed into the **AUTHORIZED** state and attempts to access an empty cellar or view their drunk history, the system will now automatically provision and store underlying empty objects upon first query. This prevents downstream apps or the client interface from running into confusing `NotFound` scenarios.

## Development

### Prerequisites
- Go 1.22+
- Docker (for integration tests)

### Commands
- Build: `go build ./...`
- Test: `go test ./...`
