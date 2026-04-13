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

**Open ideas:**

- When not sharing a screen, show the other person's camera full screen with own camera in corner
- Make cameras drag-and-droppable

### icsproxy

Calendar (ICS) filter and proxy. Takes an existing ICS feed URL, lets you hide specific events or mark holidays, and generates a new filtered calendar URL.

## Future tool ideas

### own todolist

Todoist is great but doesn't cover some specific needs:

- Recurring todos hidden until needed
- Order by priority + drag-and-drop
- Next to all existing Todoist features
- Note: mobile experience should be excellent

### proxy search engine

Appends `-AI` to every Google search to filter out AI summaries.
