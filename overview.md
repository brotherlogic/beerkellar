# Beerkellar Project Overview

Beerkellar is a command-line application and backend service designed to help users manage their personal beer cellars. It integrates with the [Untappd API](https://untappd.com/api/docs) to automatically track consumption and provide detailed beer information.

## Project Structure

- `server/`: Core backend logic implemented in Go.
  - `api.go`: gRPC service implementations for managing the cellar and user authentication.
  - `db.go`: Persistence layer using the `pstore` key-value service.
  - `untappd.go`: Integration with the Untappd API for beer data and checkins.
  - `processqueue.go`: Background job processing for tasks like caching beer metadata.
- `beerkellar_cli/`: The user-facing command-line interface.
  - Supported commands: `add` (add beer to cellar), `cellar` (view current inventory), and `login` (OAuth with Untappd).
- `proto/`: Protocol Buffer definitions for the gRPC API and data models.
  - `api.proto`: Defines the `BeerKeller` and `BeerKellerAdmin` services.
  - `beer.proto`: Defines the core `Beer` data structure.
- `fake_untappd/`: A mock/proxy server used for integration testing without hitting the real Untappd API.
- `integration/`: End-to-end integration tests for the system.

## Key Features

- **Automated Inventory Management**: When a user checks in a beer on Untappd, Beerkellar automatically identifies and removes the corresponding entry from their cellar.
- **Intelligent Selection**: The API supports picking beers based on strategies like "Oldest First" or "Random," while respecting health-conscious "units" requirements.
- **Background Metadata Fetching**: When a beer is added by ID, the system asynchronously fetches details (ABV, Brewery, Name) from Untappd to enrich the local database.
- **OAuth Integration**: Securely connects to Untappd to access user checkin feeds.

## Technical Stack

- **Language**: Go1.22+
- **Communication**: gRPC for internal and CLI-to-server communication.
- **Data Store**: Interacts with a `pstore` service for persistent storage of user profiles, cellar entries, and beer metadata.
- **Observability**: Exports Prometheus metrics on port `8081`.

## Development & CI

- **Local Development**: Uses a devcontainer environment (configured in `.devcontainer`).
- **Testing**: Includes both unit tests (`go test ./server/...`) and integration tests that use a fake Untappd backend.
- **GitHub Actions**: Automated workflows for testing and merging PRs.
