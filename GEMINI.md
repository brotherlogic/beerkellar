# Beerkellar

This project is a CLI and backend service for managing a user's beer cellar. It integrates with the [Untappd API](https://untappd.com/api/docs) to pull beer information and track check-ins.

## Tech Stack
- **Language**: Go (placed in `/server`)
- **API Strategy**: gRPC (definitions in `/proto`)
- **Storage**: `pstore` (abstraction in `server/db.go`)
- **Metrics**: Prometheus
- **Testing**: Integration tests in `/integration` with `fake_untappd` proxy.

## Project Layout
- `beerkellar_cli/`: CLI entry point and commands.
- `server/`: Backend service logic.
    - `api.go`: Main gRPC service implementation.
    - `untappd.go`: Untappd API client wrapper.
    - `processqueue.go`: Async task execution to handle API rate limits.
    - `db.go`: Persistent storage logic.
- `proto/`: Protobuf and gRPC service definitions.
- `integration/`: End-to-end integration tests.
- `fake_untappd/`: Mock server used during integration tests.

## Development Standards

### Test First
Always write a failing test first, then write the code to make it pass, then refactor. Ensure local tests cover new functionality before proceeding.

### Async Processing
The Untappd API has strict rate limiting. All background calls to external APIs must go through the `processqueue` to run asynchronously and handle retries/backoff.

### Environment
Development must occur within the project's Dev Container. The workspace root must be `/workspaces/beerkellar`. The container is configured with the correct `workingDir` to ensure consistency.

### Development Workflow
After every change, you MUST:
1. Push the change to a new feature branch.
2. Wait for GitHub to automatically create a Pull Request from the branch push (DO NOT use `gh pr create`).
3. Track and address any comments or issues raised on that PR.
4. Once the PR is submitted (merged), reset your local environment to HEAD and sync with the remote repository.

### Finishing Tasks
Once a change is complete and all tasks are done, follow the `/finish` workflow (`.agents/workflows/finish.md`). This handles building, documenting, and triggering the review process.
