# tools.xdoubleu.com

Each tool has its own folder under `apps/`, however all are combined in `cmd/publish`.
`cmd/publish` also takes care of authentication.

The folder structure of apps follows the usual one for Go projects:

- `images` — images used for app
- `migrations` — DB migrations used for app
- `templates` — HTML templates used for app
- `internal` — logic internal to that app
- `pkg` — logic that could become its own project
- root level files — HTTP endpoints of app

All apps share common bootstrapping via `internal/app.Base`, which provides logging,
config, template rendering, and auth middleware wiring.

## Development

```bash
# Start the database
docker-compose up -d

# Run all tests
make test

# Stop the database
docker-compose down

# Lint and auto-fix
make lint/fix
```

## Adding a new app

Use the scaffold command to generate a new app with all the boilerplate:

```bash
# Minimal app (no DB, no background jobs)
make scaffold NAME=myapp

# App with DB support (repositories + goose migrations)
make scaffold NAME=myapp DB=true

# App with DB and background job queue
make scaffold NAME=myapp DB=true JOBS=true
```

This generates `apps/myapp/` with the full directory structure and automatically
registers the new app in `cmd/publish/apps.go`. After scaffolding:

1. Implement handlers and register routes in `apps/myapp/routes.go`
2. Add domain logic to `apps/myapp/internal/`
3. If using DB, edit `apps/myapp/migrations/00001_init.sql` with your schema
4. Run `go build ./...` to verify

## Existing apps

### backlog

Tracks goals by pulling progress from external sources (Steam, Hardcover/Goodreads).
Supports background sync jobs and WebSocket-based live updates.

### watchparty

Real-time screen sharing and video conferencing using WebRTC.
Users create or join a room; the creator becomes the presenter and can share their screen.
Camera overlays are draggable. When no screen is being shared, the remote camera fills the main area.

### icsproxy

Calendar (ICS) filter and proxy. Takes an existing ICS feed URL, lets you hide specific events or mark holidays, and generates a new filtered calendar URL.
