# tools.xdoubleu.com

Each tool has its own folder, however all are combined in `cmd/publish`.
`cmd/publish` also takes care of authentication.

The folder structure of tools follows the usual one for Go projects:

- `images` — images used for tool
- `migrations` — DB migrations used for tool
- `templates` — HTML templates used for tool
- `internal` — logic internal to that app
- `pkg` — logic that could become its own project
- root level files — HTTP endpoints of tool

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

## Existing tools

### goaltracker

Tracks goals by pulling progress from external sources (Todoist, Steam, Goodreads).

### watchparty

Real-time screen sharing and video conferencing using WebRTC.
Users create or join a room; the creator becomes the presenter and can share their screen.
Camera overlays are draggable. When no screen is being shared, the remote camera fills the main area.

### icsproxy

Calendar (ICS) filter and proxy. Takes an existing ICS feed URL, lets you hide specific events or mark holidays, and generates a new filtered calendar URL.
