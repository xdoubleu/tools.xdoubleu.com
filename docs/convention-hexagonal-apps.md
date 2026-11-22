# Convention: hexagonal apps

Each app separates adapters (transport) from services (rules) from storage.

- **Layout:** adapters live in `apps/<app>/connect`, `apps/<app>/mcp` and `apps/<app>/http`. The root package keeps only wiring (`app.go`, routes) and the exported cross-app API.
- **Rules in services:** adapters keep only the `mcptools` gates, DTO mapping and error mapping.
- **Ports:** each service depends on interfaces declared in its own `services` package; repositories and outbound clients implement them. Domain errors a handler maps are re-exported from `services`.
- **Services import no** `connectrpc.com/connect`, `tools.xdoubleu.com/gen`, `pgx` or `net/http`.
- **Cross-app calls** go through a port the consumer declares, using domain types, never generated protos.
- **Enforcement:** two depguard rules in `.golangci.yml` cover every app by glob, so a new app is checked from its first commit: `hexagonal-adapters` (`apps/*/{connect,mcp,http}`: no pgx or `kit/database/postgres`) and `hexagonal-services` (`apps/*/internal/services`: no transport imports). depguard cannot glob an import path, so `make lint/hexagonal` (`api/scripts/check_hexagonal.sh`) bans `internal/repositories` in adapters and in the root package outside `app.go`, and any `connect` import in `mcp/` packages. The factory (`platform/factory`) has its own pair of rules plus `factory-core`: its services import only `kit` and its own models; `agentaccounts`, GitHub, Slack and auth reach them through ports that the root package adapts.
- **MCP tools** are adapters like the Connect handlers: they call services and map to DTOs through the app's `internal/protomap`, never the Connect handler.
- **Reference app:** `api/apps/shoppinglist`.
