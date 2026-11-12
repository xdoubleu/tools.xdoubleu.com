# ADR-0046: A self-hostable platform in one image, still deployed with Kamal

- Status: Accepted
- Issues: epic T-224, spike T-228, #2866
- Affects: `api/cmd/api/apps.go`, `api/internal/`, `config/`, `infra/`, `.github/workflows/main.yml`
- Changes: [adr-0001](adr-0001-two-service-kamal-deploy.md) (Kamal stays; no second service for the platform)

## Context

The apps should be self-hostable by anyone, choosing which apps to run, while the owner's instance stays family and friends only. The repo is public (GPL-3.0) and serves as an open source showcase of the platform and factory. Production is one Hetzner CX23 (4 GB RAM, 2 vCPU; about 2.5 GB available at the 7-day minimum) running two Kamal services. Server-specific deploy config lives beside the code. One broken app migration panics boot and takes the IdP, MCP and the factory down with it.

## Decision

- **Production stays on Kamal.** Kubernetes and Argo CD are not justified for one small VPS, and the two are not combined on the server.
- **Self-hosting is packaged twice:** a Docker Compose file as the main route, and a Helm chart tested in CI on a throwaway cluster. No Argo CD.
- **One image with an "enabled apps" switch**, not a deployable per app. Only enabled apps get routes, jobs, migrations, nav entries and MCP tools; an app's declared dependencies are enabled with it. The factory, MCP and observability are switches too.
- **Identity is always on.** The api always owns users, roles, app grants, family, agent accounts and the MCP authorization server. People sign in through an OIDC provider ([adr-0051](adr-0051-zitadel-for-people-sign-in.md)).
- **Two repos:** this public monorepo (platform, factory, apps, Compose file, chart), and a private repo for what is specific to the owner's server: Terraform, Kamal deploy configs, secret names, hostnames and the deploy workflow. No repo or Go module per app.
- **Code tiers in `api/`:** `internal/kit` (domain-free toolkit), `internal/platform` (identity, IdP, MCP, observability, factory) and `apps/`, with `apps/internal` for code only apps share. `kit` imports no other tier, `platform` imports only `kit`, `apps` import both, `cmd/api` wires them. An import lint enforces it.
- **A failing app migration disables that app, and boot continues.** Global and platform migrations stay fatal. A disabled app answers 503, starts no jobs and registers no MCP tools; `/health` reports degraded and names it; an `app_migration_failed{app}` gauge alerts. Its dependents are disabled too, from dependencies declared at registration. The factory becomes platform, so its migrations are fatal.
- **One SPA**; disabled apps are hidden. No module federation.

## Alternatives considered

- **Kubernetes in production**: the cluster's own footprint eats the box, for one operator.
- **A second Kamal service for the platform tier**: Kamal already swaps containers without downtime, and migration isolation covers the blast radius it was meant to limit.
- **A repo or module per app**: versioning and release overhead with one consumer.
- **Module federation**: the SPA ships as one build; hiding a disabled app costs nothing.

## Consequences

- Each app's registration declares its dependencies; the switch and migration isolation both read them.
- Deploy changes for the owner's server happen in the private repo once it exists; until then they stay in `config/` and `infra/`.
- The import lint fails a PR that crosses a tier.

## Revisit when

Production outgrows one VPS, or a second operator runs it.
