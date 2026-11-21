# Convention: hexagonal apps

Each app separates adapters (transport) from services (rules) from storage.

- **Layout:** adapters live in `apps/<app>/connect`, `apps/<app>/mcp` and `apps/<app>/http`. The root package keeps only wiring (`app.go`, routes) and the exported cross-app API.
- **Rules in services:** adapters keep only the `mcptools` gates, DTO mapping and error mapping.
- **Ports:** each service depends on interfaces declared in its own `services` package; repositories and outbound clients implement them. Domain errors a handler maps are re-exported from `services`.
- **Services import no** `connectrpc.com/connect`, `tools.xdoubleu.com/gen`, `pgx` or `net/http`.
- **Cross-app calls** go through a port the consumer declares, using domain types, never generated protos.
- **Enforcement:** depguard rules `hexagonal-adapters` (no `internal/repositories`, pgx or `kit/database/postgres` in adapters) and `hexagonal-services` in `.golangci.yml`, each listing converted apps by path.
- **Reference app:** `api/apps/shoppinglist`.
