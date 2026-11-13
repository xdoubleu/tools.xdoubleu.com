# ADR-0051: People sign in through self-hosted Zitadel; the api keeps the MCP authorization server

- Status: Accepted
- Issues: spike T-254, epic T-224
- Affects: `api/internal/platform/auth/`, `web/app/auth/`, `infra/`, `config/deploy.api.yml`
- Supersedes in part: [adr-0005](adr-0005-first-party-auth-replacing-gotrue.md) (passwords, TOTP, recovery codes, reset mail)

## Context

Auth is first-party. `internal/platform/auth` holds passwords, TOTP, recovery codes and lockout; `internal/platform/oauth2as` and `internal/platform/agentaccounts` are the MCP authorization server with dynamic client registration, client_credentials for agents and delegation at consent ([adr-0048](adr-0048-consent-delegates-to-agent-accounts.md)). The owner would rather not own password and MFA code, and wants a free provider.

No provider covers the second half: none of the hosted free tiers (Auth0, WorkOS, Stytch, Clerk, Zitadel Cloud) lets an admin switch an MCP client's subject to an agent account at consent, and Clerk has no standard client_credentials grant. Self-hosted, only Ory Hydra fits delegation by design, at the cost of writing its login and consent UI. The full evaluation is in T-254.

## Decision

- **Zitadel, self-hosted, signs people in**: passwords, TOTP, recovery, passkeys and reset mail. In production it runs beside the api on the same host with its own database in the existing Postgres; the Compose file and Helm chart ([adr-0046](adr-0046-platform-respike.md)) ship it as the default provider.
- **The api is an OIDC relying party for people.** Sign-in redirects to Zitadel; the api issues its own session from the ID token. `auth.users` links each user to the Zitadel subject. Any OIDC provider can replace Zitadel by configuration.
- **The api keeps** users, roles, app grants, family, agent accounts, delegation and the MCP authorization server. MCP tokens, agent credentials and Grafana's OIDC sign-in ([adr-0021](adr-0021-oauth-as-general-purpose-oidc-idp.md)) are unaffected.
- **User management calls Zitadel's user API** to create, invite and deactivate people; grants stay in the api.
- **Cutover imports** bcrypt hashes and TOTP secrets into Zitadel, so no one resets a password; everyone signs in once more.
- **Zitadel is pinned and patched promptly.** Renovate tracks it; a security advisory for it is handled like one for the api.

## Alternatives considered

- **Keep first-party, adding passkeys and asymmetric session signing**: the cheaper hardening, but keeps owning password and MFA code.
- **Zitadel or Ory as the MCP authorization server too**: delegation would move into a custom login app, and Zitadel's client ID metadata documents are unmerged.
- **A hosted free tier**: no delegation, DAU caps, no custom domain on Zitadel Cloud's free plan.
- **Keycloak**: about 1 GB of JVM on a 4 GB box. **Authentik**: heavier. **Pocket ID**: passkeys only. **Authelia**: users only in a file or LDAP.

## Consequences

- Sign-in depends on Zitadel being up; the box carries about 512 MB more.
- Zitadel has had several account-takeover advisories in its login UI since 2025, so its patch cadence becomes ours.
- `internal/platform/auth` shrinks to sessions issued after OIDC sign-in.

## Revisit when

A provider supports delegating a client to another subject at consent, or Zitadel's footprint or advisories outweigh owning the code.
